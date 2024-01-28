from flask import Flask, jsonify, request, abort
from prometheus_client import Histogram, Counter, generate_latest, CONTENT_TYPE_LATEST
from time import time, sleep
from datetime import datetime
import requests
import asyncio

from config import config
import psycopg2

CLIENT_ID = "b986da479ce531b"
SECRET = "027f7378b9e15c0aae455b292380a5bfba5ba71a"

SERVICE_NAME = "Imgur"
TIMEOUT_SECONDS = 5

request_duration_histogram = Histogram(
    'http_request_duration_seconds',
    'HTTP request duration in seconds',
    ['service', 'endpoint']
)

view_metric = Counter('view', 'Page view', ["endpoint"])

app = Flask(__name__)

def _check_upstream():
    imgur_api_url = "https://api.imgur.com/3/gallery/hot/viral/0.json"
    headers = {
        f"Authorization": f"Client-ID {CLIENT_ID}"
    }

    try:
        start_time = time()
        response = requests.get(imgur_api_url, headers=headers)
        end_time = time()
        response_time = end_time - start_time
        return response.status_code, response_time
    except Exception as e:
        return 500, None

def _check_database():
    conn = None
    try:
        # read connection parameters
        params = config()

        # connect to the PostgreSQL server
        print('Connecting to the PostgreSQL database...')
        conn = psycopg2.connect(**params)

        # create a cursor
        cur = conn.cursor()

        start_time = time()
        cur.execute(f'''SELECT version();''')

        # display the PostgreSQL database server version
        db_version = cur.fetchall()
        end_time = time()
        response_time = end_time - start_time
        cur.close()
        if conn is not None:
            conn.close()
            print('Database connection closed.')
        return 200, response_time

        # close the communication with the PostgreSQL
    except:
        return 500, None

@app.route("/status")
def status():
    def check_code(code):
        if code == 200:
            return "ok"
        return "error"

    db = _check_database()
    upstream = _check_upstream()
    service_status = "ok" if db[0] == 200 and upstream[0] == 200 else "error"
    response = {
        "status": service_status,
        "timestamp": datetime.utcnow().isoformat(),
        "components": [
            {
                "name": "Imgur",
                "service_status": check_code(upstream[0]),
                "request_time": upstream[1]
            },
            {
                "name": "Database",
                "service_status": check_code(db[0]),
                "request_time": db[1]
            }
        ]

    }
    return jsonify(response)

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

@app.route('/tag/<string:tag>')
def search_for_tag(tag):
    start_time = time()
    tagName, sort, window, page = tag, "top", "all", 1
    url = f"https://api.imgur.com/3/gallery/t/{tagName}/{sort}/{window}/{page}"

    headers = {
        f"Authorization": f"Client-ID {CLIENT_ID}"
    }

    response = requests.get(url, headers=headers)
    end_time = time()
    request_duration_histogram.labels(service=SERVICE_NAME, endpoint='tags').observe(end_time - start_time)
    view_metric.labels(endpoint="tags").inc()
    return response.json(), response.status_code

@app.route('/album')
def create_album():
    start_time = time()
    url = 'https://api.imgur.com/3/album'
    headers = {
        f"Authorization": f"Client-ID {CLIENT_ID}"
    }

    data = {
        'title': request.form.get("title"),
        'description': request.form.get("description"),
    }

    response = requests.post(url, headers=headers, data=data)

    end_time = time()
    request_duration_histogram.labels(service=SERVICE_NAME, endpoint='album').observe(end_time - start_time)
    view_metric.labels(endpoint="album").inc()
    return response.json(), response.status_code

@app.route('/upload')
def upload_image():
    start_time = time()
    url = 'https://api.imgur.com/3/image'
    headers = {
        f"Authorization": f"Client-ID {CLIENT_ID}"
    }

    data = {
        'image': request.form.get("image"),
        'album': request.form.get("album"),
        'type': "url",
        'name': request.form.get("name"),
        'title': request.form.get("description"),
        'description': request.form.get("description"),
        'disable_audio': 1
    }

    response = requests.post(url, headers=headers, data=data)

    end_time = time()
    request_duration_histogram.labels(service=SERVICE_NAME, endpoint='upload').observe(end_time - start_time)
    view_metric.labels(endpoint="upload").inc()
    return response.json(), response.status_code


if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')