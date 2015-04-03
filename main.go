package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/alexurquhart/qapi"
	_ "github.com/mattn/go-sqlite3"
)

type SP500Symbol struct {
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Industry    string `json:"industry"`
	SubIndustry string `json:"subindustry"`
	Exchange    string `json:"exchange"`
	SymbolID    int
	Candles     []qapi.Candlestick
}

// Extract candlestick data over 5 years for a given symbol.
func extractCandles(c *qapi.Client, t *time.Ticker, id int) ([]qapi.Candlestick, error) {
	<-t.C
	candles, err := c.GetCandles(id, time.Now().AddDate(-5, 0, 0), time.Now(), "OneDay")
	if err != nil {
		return []qapi.Candlestick{}, err
	}

	return candles, nil
}

// Find data for the symbol - first the internal symbol identifier needs to be found
// then candlestrick data is extracted. The result should then be saved to a database
func findSymbol(c *qapi.Client, t *time.Ticker, sym *SP500Symbol) error {
	<-t.C
	res, err := c.SearchSymbols(sym.Symbol, 0)
	if err != nil {
		return err
	}

	// Find the symbol and extract the symbol ID
	for _, r := range res {
		// If the symbol is a match - extract candles
		if r.Symbol == sym.Symbol && r.ListingExchange == sym.Exchange {
			candles, err := extractCandles(c, t, r.SymbolID)
			if err != nil {
				return err
			}
			sym.SymbolID = r.SymbolID
			sym.Candles = candles
			return nil
		}
	}
	return errors.New("Symbol not found: " + sym.Symbol)
}

// Starts a goroutine that iterates over a channel of incoming
// symbols. Returns an error channel.
func saveData(wg *sync.WaitGroup, symChan chan SP500Symbol) chan error {
	errChan := make(chan error)
	go func(wg *sync.WaitGroup, errChan chan error, symChan chan SP500Symbol) {
		defer close(errChan)

		// Open a database connection
		db, err := sql.Open("sqlite3", "sp500.db")
		if err != nil {
			errChan <- err
			return
		}
		defer db.Close()

		// Read the schema file and create the database
		file, _ := ioutil.ReadFile("schema.sql")
		_, err = db.Exec(string(file))
		if err != nil {
			errChan <- err
			return
		}

		// Prepare statements to insert data into the symbol and candlestick tables
		symStmt, err := db.Prepare("insert into symbolids values (?, ?, ?, ?, ?, ?)")
		if err != nil {
			errChan <- err
			return
		}
		defer symStmt.Close()
		cdlStmt, err := db.Prepare("insert into candlestick values(?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			errChan <- err
			return
		}
		defer cdlStmt.Close()

		// Iterate over all incoming symbols
		for sym := range symChan {
			tx, _ := db.Begin()
			_, err = symStmt.Exec(sym.SymbolID, sym.Symbol, sym.Exchange, sym.Name, sym.Industry, sym.SubIndustry)
			if err != nil {
				errChan <- err
			}

			for _, cdl := range sym.Candles {
				_, err := cdlStmt.Exec(sym.SymbolID, cdl.Start, cdl.End, cdl.Open, cdl.Close, cdl.High, cdl.Low, cdl.Volume)
				if err != nil {
					errChan <- err
				}
			}
			err := tx.Commit()
			if err != nil {
				errChan <- err
			}

		}
		wg.Done()
	}(wg, errChan, symChan)
	return errChan
}

func main() {
	// Read in the JSON file of S&P 500 Symbols and their exchanges
	file, _ := ioutil.ReadFile("sp500.json")
	var symbols []SP500Symbol
	err := json.Unmarshal(file, &symbols)
	if err != nil {
		log.Fatal(err)
	}

	// Login to the server using the refresh token stored
	// in the environment variables
	refresh := os.Getenv("REFRESH_TOKEN")
	client, err := qapi.NewClient(refresh, false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("export REFRESH_TOKEN=" + client.Credentials.RefreshToken + "\n\n")

	// Create a rate limting ticker - Questrade limits market calls to 5 per second
	// up to 15 000 calls per hour. Lets set a delay of 250 ms - which will get us
	// 14 400 calls per hour at 4 requests per second
	interval := 250 * time.Millisecond
	ticker := time.NewTicker(interval)

	// Create a new wait group so that main will block until all goroutines
	// are finished (saving to the database takes awhile)
	var wg sync.WaitGroup
	wg.Add(2)

	// Create a channel for the populated symbol structs to be sent over
	// to be saved to the database.
	symChan := make(chan SP500Symbol)
	errChan := saveData(&wg, symChan)
	stopChan := make(chan bool)

	// Create a new map that will hold symbols that could not be found
	notFound := make([]SP500Symbol, 1)

	// Separate goroutine to output database write errors
	go func(wg *sync.WaitGroup, errChan chan error) {
		for err := range errChan {
			log.Println("DB Error: ", err)
		}
		log.Println("DB error logging stopped.")
		close(stopChan)
		wg.Done()
	}(&wg, errChan)

L:
	for _, sym := range symbols {
		select {
		case <-client.SessionTimer.C: // Login to the practice server again when session expires
			log.Println("Logging in again...")
			client.Login(false)
			break
		case _, ok := <-stopChan: // Break the loop if a critical DB error occurs in the other goroutine
			if !ok {
				break L
			}
			break
		default:
			err := findSymbol(client, ticker, &sym)
			if err != nil {
				notFound = append(notFound, sym)
				log.Printf("Could not find symbol %s\n", sym.Symbol)
				break
			}
			log.Printf("Retreived %d candles for %s\n", len(sym.Candles), sym.Symbol)
			symChan <- sym
			break
		}
	}
	close(symChan)
	log.Println("Waiting for data to be saved...")
	wg.Wait()

	// Output list of symbols not found
	log.Printf("%d Symbols Not Saved", len(notFound))
	for _, e := range notFound {
		log.Println(e.Symbol)
	}
	log.Println("export REFRESH_TOKEN=" + client.Credentials.RefreshToken + "\n\n")
}
