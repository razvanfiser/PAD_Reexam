from flask import Flask, jsonify, request
from prometheus_client import Histogram, Counter, generate_latest, CONTENT_TYPE_LATEST
from time import time
import requests

# from config import config
import psycopg2

CLIENT_ID = "b986da479ce531b"
SECRET = "027f7378b9e15c0aae455b292380a5bfba5ba71a"

app = Flask(__name__)

@app.route('/search')
def get_event_at_date():
    sort, window, page = "time", "all", 1
    query = request.args.get("q")
    url = f"https://api.imgur.com/3/gallery/search/{sort}/{page}?q_all={query}"

    headers = {
        f"Authorization": f"Client-ID {CLIENT_ID}"
    }

    response = requests.get(url, headers=headers)

    return response.json(), response.status_code

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')