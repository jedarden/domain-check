#!/bin/bash
# Memory growth test: 50 req/s for 10 minutes
# Uses randomized IPs to avoid per-IP rate limiting
# Total requests: 30,000

set -e

SERVER_URL="http://localhost:8082"
RATE=50
DURATION=600s
METRICS_URL="${SERVER_URL}/metrics"
RESULTS_DIR="memory-test-results"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

mkdir -p "$RESULTS_DIR"

echo "=== Memory Growth Test ===" | tee "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo "Rate: $RATE req/s" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo "Duration: $DURATION" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo "Total requests: $((RATE * 600))" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo "Start time: $(date)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo ""

# Get initial memory baseline (use awk for proper float handling)
echo "Getting initial memory baseline..."
curl -s "$METRICS_URL" > "$RESULTS_DIR/metrics-initial-${TIMESTAMP}.txt"
INITIAL_HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
# Convert scientific notation to integer using awk
INITIAL_HEAP_MB=$(echo "$INITIAL_HEAP" | awk '{printf "%.0f", $1/1024/1024}')
echo "Initial heap: $INITIAL_HEAP bytes (${INITIAL_HEAP_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Start vegeta attack
echo "Starting vegeta attack..."
echo "GET http://localhost:8082/api/v1/check?d=example.com" | \
  vegeta attack -rate=$RATE/s -duration=$DURATION \
    -header="X-Forwarded-For: 192.168.1.1" \
    -header="X-Real-IP: 192.168.1.1" \
    -name=memory-test > "$RESULTS_DIR/vegeta-results-${TIMESTAMP}.bin" 2>&1 &
VEGETA_PID=$!

# Monitor memory every 30 seconds
echo ""
echo "Monitoring memory every 30 seconds..."
echo "Time,Heap_Bytes,Heap_MB,Heap_Objects,Stack_Bytes,Goroutines" > "$RESULTS_DIR/memory-timeseries-${TIMESTAMP}.csv"

ELAPSED=0
while kill -0 "$VEGETA_PID" 2>/dev/null; do
  METRICS=$(curl -s "$METRICS_URL")
  HEAP=$(echo "$METRICS" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
  HEAP_MB=$(echo "$HEAP" | awk '{printf "%.2f", $1/1024/1024}')
  OBJS=$(echo "$METRICS" | grep "^go_memstats_heap_objects" | awk '{print $2}')
  STACK=$(echo "$METRICS" | grep "^go_memstats_stack_inuse_bytes" | awk '{print $2}')
  GOROS=$(echo "$METRICS" | grep "^go_goroutines" | awk '{print $2}')

  echo "${ELAPSED}s,${HEAP},${HEAP_MB},${OBJS},${STACK},${GOROS}" >> "$RESULTS_DIR/memory-timeseries-${TIMESTAMP}.csv"
  echo "[${ELAPSED}s] Heap: ${HEAP_MB} MB, Objects: ${OBJS}, Goroutines: ${GOROS}"

  sleep 30
  ELAPSED=$((ELAPSED + 30))
done

wait $VEGETA_PID || echo "Vegeta exited with code $?"

# Get final memory
echo ""
echo "Getting final memory..."
curl -s "$METRICS_URL" > "$RESULTS_DIR/metrics-final-${TIMESTAMP}.txt"
FINAL_HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
FINAL_HEAP_MB=$(echo "$FINAL_HEAP" | awk '{printf "%.0f", $1/1024/1024}')
echo "Final heap: $FINAL_HEAP bytes (${FINAL_HEAP_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Calculate growth using awk for float handling
GROWTH_MB=$(echo "$INITIAL_HEAP $FINAL_HEAP" | awk '{printf "%.2f", ($2 - $1)/1024/1024}')
echo "Heap growth: ${GROWTH_MB} MB" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Report vegeta results
echo ""
echo "=== Vegeta Report ===" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
if [ -f "$RESULTS_DIR/vegeta-results-${TIMESTAMP}.bin" ]; then
    vegeta report "$RESULTS_DIR/vegeta-results-${TIMESTAMP}.bin" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
else
    echo "No vegeta results file found" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
fi

echo ""
echo "=== Test Summary ===" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo "Results saved to: $RESULTS_DIR/" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Check if growth is acceptable
GROWTH_INT=$(echo "$GROWTH_MB" | awk '{printf "%d", $1}')
if [ $GROWTH_INT -lt 100 ]; then
  echo "✓ Memory growth is acceptable (<100 MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
  exit 0
else
  echo "✗ WARNING: Memory growth exceeded 100 MB (${GROWTH_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
  exit 1
fi
