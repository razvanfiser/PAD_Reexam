from flask import Flask, jsonify, request
from prometheus_client import Histogram, Counter, generate_latest, CONTENT_TYPE_LATEST
from time import time
import requests

from config import config
import psycopg2

CLIENT_ID = "b986da479ce531b"
SECRET = "027f7378b9e15c0aae455b292380a5bfba5ba71a"

SERVICE_NAME = "Imgur"

request_duration_histogram = Histogram(
    'http_request_duration_seconds',
    'HTTP request duration in seconds',
    ['service', 'endpoint']
)

view_metric = Counter('view', 'Page view', ["endpoint"])

app = Flask(__name__)

@app.route('/metrics')
def metrics():
    return generate_latest(), 200, {'Content-Type': CONTENT_TYPE_LATEST}

def _insert_into_db(links):
    conn = None
    try:
        # read connection parameters
        params = config()

        # connect to the PostgreSQL server
        print('Connecting to the PostgreSQL database...')
        conn = psycopg2.connect(**params)

        # create a cursor
        cur = conn.cursor()

        str_to_execute = 'INSERT INTO images (link) VALUES ' + ", ".join([f"('{link}')" for link in links]) + ";"
        # str_to_execute = f"INSERT INTO images (link) VALUES ('loooool');"
        print(str_to_execute, flush=True)
        cur.execute(str_to_execute)
        conn.commit()
        # display the PostgreSQL database server version
        cur.close()
        if conn is not None:
            conn.close()
            print('Database connection closed.')
        return "OK", 200

        # close the communication with the PostgreSQL

    except (Exception, psycopg2.DatabaseError) as error:
        return jsonify({"Database Error": str(error)}), 500

@app.route("/get_db")
def get_db():
    conn = None
    try:
        # read connection parameters
        params = config()

        # connect to the PostgreSQL server
        print('Connecting to the PostgreSQL database...')
        conn = psycopg2.connect(**params)

        # create a cursor
        cur = conn.cursor()

        cur.execute(f'''SELECT * FROM images;''')

        # display the PostgreSQL database server version
        db_version = cur.fetchall()
        cur.close()
        if conn is not None:
            conn.close()
            print('Database connection closed.')
        return jsonify(db_version), 200

        # close the communication with the PostgreSQL

    except (Exception, psycopg2.DatabaseError) as error:
        return jsonify({"Database Error": str(error)}), 500

@app.route('/search')
def search_for_term():
    start_time = time()
    sort, window, page = "top", "all", 1
    query = request.args.get("q")
    url = f"https://api.imgur.com/3/gallery/search/{sort}/{page}?q_all={query}"

    headers = {
        f"Authorization": f"Client-ID {CLIENT_ID}"
    }

    response = requests.get(url, headers=headers)
    # print(response.status_code, response.json())
    data = response.json()["data"]
    out = {"images": [data[i]["link"] for i in range(len(data))]}

    msg, status = _insert_into_db(out["images"])
    print(f"{msg}, {status}", flush=True)

    end_time = time()
    request_duration_histogram.labels(service=SERVICE_NAME, endpoint='search_photo').observe(end_time - start_time)
    view_metric.labels(endpoint="search_photo").inc()
    return jsonify(out), response.status_code



if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')