from flask import Flask
from config import config
import psycopg2

app = Flask(__name__)

@app.route('/')
def hello_world():
    return '<h1>Sports API Server</h1>'

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')
