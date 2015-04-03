CREATE TABLE symbolids (
    "id" INTEGER PRIMARY KEY NOT NULL,
    "symbol" TEXT NOT NULL,
    "exchange" TEXT NOT NULL,
    "name" text not null,
    "industry" text not null,
    "subindustry" text not null
);
CREATE TABLE candlestick (
    "id" INTEGER NOT NULL,
    "starttime" DATETIME NOT NULL,
    "endtime" DATETIME NOT NULL,
    "open" REAL NOT NULL,
    "close" REAL NOT NULL,
    "high" REAL NOT NULL,
    "low" REAL NOT NULL,
    "volume" INTEGER NOT NULL,
    foreign key(id) references symbolids(id)
);
CREATE INDEX "i_candlestick" on candlestick (id ASC, starttime DESC, endtime DESC);
