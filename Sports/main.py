from flask import Flask, jsonify
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

@app.route('/_getsportlist')
def _get_sport_list():
    url = "https://sportscore1.p.rapidapi.com/sports"

    headers = {
        "X-RapidAPI-Key": "6e297b3aa8mshee38d1a1c1c6f1ep11483ejsn6602bee3a178",
        "X-RapidAPI-Host": "sportscore1.p.rapidapi.com"
    }

    response = requests.get(url, headers=headers)

    return response.json()

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

@app.route('/events/<int:sport_id>/live', methods=["GET"])
def get_live_events(sport_id):
    url = f"https://sportscore1.p.rapidapi.com/sports/{sport_id}/events/live"

    querystring = {"page": "1"}

    headers = {
        "X-RapidAPI-Key": "6e297b3aa8mshee38d1a1c1c6f1ep11483ejsn6602bee3a178",
        "X-RapidAPI-Host": "sportscore1.p.rapidapi.com"
    }

    response = requests.get(url, headers=headers, params=querystring)
    r_json = response.json()["data"]
    out = [{"away_score": r_json[i]["away_score"],
           "home_score": r_json[i]["home_score"],
           "away_team": r_json[i]["away_team"],
           "home_team": r_json[i]["home_team"]} for i in range(len(r_json))]
    return jsonify(out), response.status_code

@app.route('/events/date/<string:date>', methods=["GET"])
def get_event_at_date(date):
    url = f"https://sportscore1.p.rapidapi.com/events/date/{date}"

    querystring = {"page": "1"}

    headers = {
        "X-RapidAPI-Key": "6e297b3aa8mshee38d1a1c1c6f1ep11483ejsn6602bee3a178",
        "X-RapidAPI-Host": "sportscore1.p.rapidapi.com"
    }

    response = requests.get(url, headers=headers, params=querystring)
    r_json = response.json()["data"]
    out = [{"away_score": r_json[i]["away_score"],
            "home_score": r_json[i]["home_score"],
            "away_team": r_json[i]["away_team"],
            "home_team": r_json[i]["home_team"]} for i in range(len(r_json))]
    return jsonify(out), response.status_code

@app.route('/teams/<int:sport_id>', methods=["GET"])
def get_teams_by_id(sport_id):
    url = f"https://sportscore1.p.rapidapi.com/sports/{sport_id}/teams"

    querystring = {"page": "1"}

    headers = {
        "X-RapidAPI-Key": "6e297b3aa8mshee38d1a1c1c6f1ep11483ejsn6602bee3a178",
        "X-RapidAPI-Host": "sportscore1.p.rapidapi.com"
    }

    response = requests.get(url, headers=headers, params=querystring)
    r_json = response.json()["data"]
    out = [{"name": r_json[i]["name"],
            "country": r_json[i]["country"],
            "gender": r_json[i]["gender"],
            "sport_id": r_json[i]["sport_id"]} for i in range(len(r_json))]
    return jsonify(out), response.status_code

@app.route('/players/<int:sport_id>', methods=["GET"])
def get_players_by_id(sport_id):
    url = f"https://sportscore1.p.rapidapi.com/sports/{sport_id}/players"

    querystring = {"page": "1"}

    headers = {
        "X-RapidAPI-Key": "6e297b3aa8mshee38d1a1c1c6f1ep11483ejsn6602bee3a178",
        "X-RapidAPI-Host": "sportscore1.p.rapidapi.com"
    }

    response = requests.get(url, headers=headers, params=querystring)
    r_json = response.json()["data"]
    out = [{"name": r_json[i]["name"],
            "main_team": r_json[i]["main_team"],
            "date_of_birth": r_json[i]["date_birth_at"],
            "height": r_json[i]["height"],
            "rating": r_json[i]["rating"],
            "market_value": r_json[i]["market_value"],
            "sport_id": r_json[i]["sport_id"]} for i in range(len(r_json))]
    return jsonify(out), response.status_code

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')