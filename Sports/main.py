from flask import Flask
from prometheus_client import Histogram, Counter, generate_latest, CONTENT_TYPE_LATEST
from time import time
import requests

from config import config
import psycopg2

app = Flask(__name__)

request_duration_histogram = Histogram(
    'http_request_duration_seconds',
    'HTTP request duration in seconds',
    ['method', 'endpoint']
)

view_metric = Counter('view', 'Page view', ["endpoint"])

@app.route('/')
def hello_world():
    start_time = time()
    url = "https://sportscore1.p.rapidapi.com/sports"

    headers = {
        "X-RapidAPI-Key": "6e297b3aa8mshee38d1a1c1c6f1ep11483ejsn6602bee3a178",
        "X-RapidAPI-Host": "sportscore1.p.rapidapi.com"
    }

    response = requests.get(url, headers=headers)

    end_time = time()
    request_duration_histogram.labels(method='GET', endpoint='/').observe(end_time - start_time)
    view_metric.labels(endpoint="/").inc()
    return response.json()

@app.route('/metrics')
def metrics():
    return generate_latest(), 200, {'Content-Type': CONTENT_TYPE_LATEST}


if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')
