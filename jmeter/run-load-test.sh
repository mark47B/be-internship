#!/bin/bash
set -euxo pipefail

# значения по-умолчанию (если не заданы в docker-compose)
THREADS="${THREADS:-10}"
RAMP_UP="${RAMP_UP:-10}"
LOOPS="${LOOPS:-100}"
HOST="${HOST:-app}"
PORT="${PORT:-8080}"

TEST_PLAN="/tests/load-test.jmx"
RESULTS="/reports/results.csv"
HTML_REPORT_DIR="/reports/html-report"
JMETER_LOG="/reports/jmeter.log"

mkdir -p "$(dirname "$RESULTS")"
rm -rf "$HTML_REPORT_DIR"
mkdir -p "$HTML_REPORT_DIR"

echo "========================================="
echo "Threads: $THREADS"
echo "Ramp-up: $RAMP_UP seconds"
echo "Loops: $LOOPS"
echo "Host: $HOST"
echo "Port: $PORT"
echo "========================================="

# Обычная проверка доступности приложения (можно оставить)
echo "Waiting for app to be ready..."
for i in {1..30}; do
  if curl -s "http://$HOST:$PORT/health" | grep -q '"status":"ok"'; then
    echo "App is ready!"
    break
  fi
  echo "waiting..."
  sleep 1
done

echo "Starting JMeter load test..."

jmeter -n -t "$TEST_PLAN" \
  -l "$RESULTS" \
  -j "$JMETER_LOG" \
  -JTHREADS="$THREADS" -JRAMP_UP="$RAMP_UP" -JLOOPS="$LOOPS" -JHOST="$HOST" -JPORT="$PORT"

echo "Load test finished. Results saved to: $RESULTS"
echo "Checking results before generating HTML report..."

# Проверяем, есть ли реальные сэмплы (не нулевой размер и есть строки с данными)
if [ -s "$RESULTS" ] && grep -q "time," "$RESULTS"; then
  echo "Generating HTML report..."
  jmeter -g "$RESULTS" -o "$HTML_REPORT_DIR" -j "$JMETER_LOG" -f
  echo "HTML Report generated: $HTML_REPORT_DIR/index.html"
else
  echo "No samples found in $RESULTS — skipping HTML report generation."
  echo "Tail of jmeter.log:"
  tail -n 200 "$JMETER_LOG" || true
fi

echo "Load test completed!"
