package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/kevin-cantwell/remora/pkg/blockchain"
	_ "github.com/mattn/go-sqlite3"
	coinbasepro "github.com/preichenberger/go-coinbasepro/v2"
)

func main() {
	if _, ok := os.LookupEnv("COINBASE_PRO_KEY"); !ok {
		log.Fatalln("Must set COINBASE_PRO_KEY in environment")
	}
	if _, ok := os.LookupEnv("COINBASE_PRO_PASSPHRASE"); !ok {
		log.Fatalln("Must set COINBASE_PRO_PASSPHRASE in environment")
	}
	if _, ok := os.LookupEnv("COINBASE_PRO_SECRET"); !ok {
		log.Fatalln("Must set COINBASE_PRO_SECRET in environment")
	}

	blockchainAPIKey, ok := os.LookupEnv("BLOCKCHAIN_API_KEY")
	if !ok {
		log.Fatalln("Must set BLOCKCHAIN_API_KEY in environment")
	}

	// Allows us to share a single http client across both apis. So we
	// can enforce global timeouts, etc.
	httpClient := setupHttpClient()

	db, err := newDB("database.db")
	if err != nil {
		log.Fatalln(err)
	}

	// COINBASE client setup
	//
	// Configurations loaded from env vars.
	// To use the sandbox environment, set COINBASE_PRO_SANDBOX=1
	coinbaseClient := coinbasepro.NewClient()
	coinbaseClient.HTTPClient = httpClient

	go startCoinbaseAction(db, coinbaseClient)

	// BLOCKCHAIN client setup
	//
	// Question: Is there a dev url we can use?
	blockchainClient, err := blockchain.NewClient(
		"",
		blockchain.WithBaseURL("https://api.blockchain.com/v3/exchange"),
		blockchain.WithHTTPClient(&blockchainRequestAuthenticator{
			apiKey:     blockchainAPIKey,
			httpClient: *httpClient,
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}

	startBlockchainAction(db, blockchainClient)
}

func setupHttpClient() *http.Client {
	return http.DefaultClient
}

type blockchainRequestAuthenticator struct {
	apiKey     string
	httpClient http.Client
}

func (doer *blockchainRequestAuthenticator) Do(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-API-Token", doer.apiKey)
	return doer.httpClient.Do(req)
}

type sqlite struct {
	r *sql.DB // concurrent reader
	w *sql.DB // synchronous writer
}

func newDB(dsn string) (*sqlite, error) {
	w, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	// sqlite3 does not allow concurrent write connections
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(-1)

	r, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	// sqlite3 allows concurrent readers
	r.SetMaxOpenConns(50)
	r.SetMaxIdleConns(10)
	r.SetConnMaxLifetime(-1)

	return &sqlite{
		w: w,
		r: r,
	}, nil
}
