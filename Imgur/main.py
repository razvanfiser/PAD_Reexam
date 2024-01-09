from flask import Flask, jsonify, request
from prometheus_client import Histogram, Counter, generate_latest, CONTENT_TYPE_LATEST
from time import time
import requests
import urllib

# from config import config
import psycopg2

CLIENT_ID = "b986da479ce531b"
SECRET = "027f7378b9e15c0aae455b292380a5bfba5ba71a"

app = Flask(__name__)

@app.route('/search')
def search_for_term():
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

    return jsonify(out), response.status_code

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')