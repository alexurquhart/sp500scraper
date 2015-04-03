#S&P 500 Symbol Scraper

This is an example program, written in Go, that queries 1 day resolution candlestick data for
symbols belonging to the S&P 500 from the Questrade API, and saves the result into an 
SQLite database. This program uses my wrapper over the Questrade API, [qapi](https://github.com/alexurquhart/qapi).

##Getting Started
To start, you will need to be able to access the [Questrade API](http://www.questrade.com/api)

To run the example, you will need to store your refresh token in the environment variable REFRESH_TOKEN
```bash
export REFRESH_TOKEN=<your token here>
```

##Dependencies
```
go get github.com/alexurquhart/qapi
go get github.com/mattn/go-sqlite3
```

##Notes
The symbols stored in sp500.json were scraped from Wikipedia on March 31st 2015 - this program
does not take into account when symbols were added or removed from the index. It scrapes all symbols
in the list back to a maximum of 5 years.
