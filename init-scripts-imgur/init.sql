-- init.sql
CREATE TABLE images (
  id serial PRIMARY KEY,
  link VARCHAR (100) NOT NULL,
  timestamp_column TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
