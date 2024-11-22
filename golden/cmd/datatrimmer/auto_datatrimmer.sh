#!/bin/bash

# Automate the data trimming process of multiple batches. This script calls
# datatrimmer in a loop, and compares batch size and the delete row count to
# determine the last batch to exit the loop.
#
# Usage:
# ./auto_datatrimmer.sh --db_name=skiainfra --table_name=tiledtracedigests \
#     --batch_size=100 --dry_run=false --cut_off_date=2021-02-15
# Note that all arguments will be passed to datatrimmer as is. The script only
# extracts out the batch_size for its comparison.

# Define a default batch size
batchsize=1000

# Save all arguments before processing
all_args=("$@")

# Parse command-line arguments to get the provided batch size
while [[ $# -gt 0 ]]; do
  case $1 in
    --batch_size=*)
      batchsize="${1#*=}"   # Extract value after the '=' sign
      shift # past argument
      ;;
    *)
      shift  # Just shift without saving
      ;;
  esac
done

echo "Start processing: actual rows count might be > batch size $batchsize."

# Initialize iteration counter
iteration=1

# Loop until last batch or error
while true; do

    # Execute command with batch size and save output
    # Override batch_size as the default value might be different in datatrimmer
    log_file="datatrimmer_log_$iteration.txt"
    bazelisk run //golden/cmd/datatrimmer -- "${all_args[@]}" \
        --batch_size="$batchsize" > "$log_file" 2>&1

    # Check for errors (stderr)
    if [ $? -ne 0 ]; then
        echo "Error occurred @ batch (iteration $iteration). Exiting." >&2
        cat "$log_file" >&2 # Print the log file to stderr for debugging
        break
    fi

    # Parse output file to get the number of inserted rows (to the temp table)
    insert_count=$(grep -oP "DELETE \K\d+" "$log_file")

    # Compare and exit if necessary
    if [ -z "$insert_count" ]; then
        echo "No inserts detected. Exiting loop."
        break
    fi
    echo "Processed batch $iteration: $insert_count+ rows have been trimmed"

    if [ "$insert_count" -lt "$batchsize" ]; then
        echo "Last batch processed. Exiting loop."
        break
    fi

    # Increment iteration counter
    iteration=$((iteration + 1))

    # Wait for 10 seconds before next iteration
    sleep 10
done

echo "Goodbye."
