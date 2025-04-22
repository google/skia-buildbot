"""Script to update data for training and experiment.

To run the script, you need to first follow the instruction to install venv.
Then run
```
source ~/venv/bin/activate
```
"""
import argparse
import base64
import datetime
import json
import sys

from google.api_core import exceptions
from google.cloud import spanner

# This is the experiment instance.
INSTANCE_ID = "tfgen-spanid-20250415224933743"
DATABASE_ID = "testing"


def insert(transaction, values):
  """Insert into experiment table."""
  transaction.insert_or_update(
      'tracevalues',
      columns=('trace_id', 'commit_number', 'val', 'createdat'),
      values=values)


def insert_training(transaction, values):
  """Insert into training table."""
  transaction.insert_or_update(
      'training',
      columns=('trace_id', 'commit_number', 'val',
               'createdat', 'is_anomaly', 'data_type'),
      values=values)


def read_json(json_string):
  try:
    data = json.loads(json_string)
    return data
  except json.JSONDecodeError:
    print(
        f"Error: Could not decode JSON from the file '{file_path}'. Please ensure the file contains valid JSON.")
    return None


def read_json_from_file(file_path):
  """
  Reads JSON data from the specified file and returns it as a Python dictionary.

  Args:
      file_path (str): The path to the JSON file.

  Returns:
      dict: The JSON data as a Python dictionary, or None if an error occurs.
  """
  try:
    with open(file_path, 'r') as f:
      data = json.load(f)
    return data
  except FileNotFoundError:
    print(f"Error: The file '{file_path}' was not found.")
    return None
  except json.JSONDecodeError:
    print(
        f"Error: Could not decode JSON from the file '{file_path}'. Please ensure the file contains valid JSON.")
    return None
  except Exception as e:
    print(f"An unexpected error occurred while reading the file: {e}")
    return None


def gen_values(key, entry):
  """Get values for tables.

  Args:
    key: Str for the key.
    entry: Dict for the json object.

  Returns:
    (values, training) Two lists. One is for the experiment table. One is for the training table.
  """
  text_bytes = key.encode('utf-8')  # Encode the string to bytes
  encoded_bytes = base64.b64encode(text_bytes)
  epoch_seconds = 1681699200
  values = []
  training = []
  i = 0
  for val in entry['vals']:
    # 1 hour intervals. Some model require at least 1 min intervals to proceed.
    timestamp = epoch_seconds + i * 60 * 60
    utc_datetime = datetime.datetime.fromtimestamp(timestamp)
    timestamp_string = utc_datetime.isoformat(timespec='seconds') + 'Z'
    is_anomaly = 0
    data_type = 'TRAIN'
    if len(entry['change_point_pos']) == 1:
      if i >= entry['change_point_pos'][0] and i < entry['change_point_pos'][0] + entry['num_change_point']:
        is_anomaly = 1
    else:
      if i in entry['change_point_pos']:
        is_anomaly = 1
    # 10% data for validation. 10% for testing. 80% for training.
    if entry['sample_index'] % 10 == 1:
      data_type = 'VALIDATE'
    elif entry['sample_index'] % 10 == 2:
      data_type = 'TEST'
    values.append((encoded_bytes, i, float(val), timestamp_string))
    training.append((encoded_bytes, i, float(
        val), timestamp_string, is_anomaly, data_type))
    i = i + 1
  return (values, training)


def process_json_data(args, json_data):
  spanner_client = spanner.Client()
  instance = spanner_client.instance(INSTANCE_ID)
  database = instance.database(DATABASE_ID)

  for key in json_data.keys():
    entry = read_json(json_data[key])
    values, training = gen_values(key, entry)
    try:
      if not args.no_exp:
        database.run_in_transaction(insert, values=values)
      if not args.no_training:
        database.run_in_transaction(insert_training, values=training)
    except exceptions.AlreadyExists:
      print('exist')


def parse_args():
  parser = argparse.ArgumentParser(
      description="A sample script with command-line flags.")
  parser.add_argument("-f", "--file", help="The input file name")
  parser.add_argument("--no-training", action="store_true",
                      help="Do not update training table")
  parser.add_argument("--no-exp", action="store_true",
                      help="Do not update experiment table")
  args = parser.parse_args()
  return args


def main():
  """
  Main function to handle command-line argument and process the JSON file.
  """
  args = parse_args()
  file_path = args.file
  json_data = read_json_from_file(file_path)
  process_json_data(args, json_data)


if __name__ == "__main__":
  main()
