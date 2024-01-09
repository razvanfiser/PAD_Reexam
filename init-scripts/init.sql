-- init.sql
CREATE TABLE sports (
  sport_id serial PRIMARY KEY,
  sport_name VARCHAR (100) NOT NULL
);

INSERT INTO sports (sport_name)
VALUES
       ('football'),
       ('tennis'),
       ('basketball'),
       ('ice+hockey'),
       ('volleyball'),
       ('handball');
