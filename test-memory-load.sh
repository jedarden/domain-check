#!/bin/bash
# Memory growth test: 50 req/s for 10 minutes
# Uses randomized IPs to avoid per-IP rate limiting
# Total requests: 30,000

set -e

SERVER_URL="http://localhost:8080"
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

# Get initial memory baseline
echo "Getting initial memory baseline..."
curl -s "$METRICS_URL" | grep "^go_memstats" | sort > "$RESULTS_DIR/metrics-initial-${TIMESTAMP}.txt"
INITIAL_HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
INITIAL_HEAP_MB=$((INITIAL_HEAP / 1024 / 1024))
echo "Initial heap: $INITIAL_HEAP bytes (${INITIAL_HEAP_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Create vegeta attack with randomized IPs (using headers)
echo "Starting vegeta attack with randomized IPs..."
cat > /tmp/vegeta-attack.sh << 'EOF'
#!/bin/bash
# Generate random IP for X-Forwarded-For header
generate_random_ip() {
    echo "$((RANDOM % 256)).$((RANDOM % 256)).$((RANDOM % 256)).$((RANDOM % 256))"
}

while true; do
    IP=$(generate_random_ip)
    echo "GET http://localhost:8080/api/v1/check?d=test-$(openssl rand -hex 4).com"
    sleep 0.02
done
EOF
chmod +x /tmp/vegeta-attack.sh

# Use vegeta with custom headers to simulate different IPs
# We'll use multiple targets to vary the domain
for i in {1..100}; do
  echo "GET http://localhost:8080/api/v1/check?d=test-${i}-example.com"
done > /tmp/targets.txt

echo "GET http://localhost:8080/api/v1/check?d=example.com" | \
  vegeta attack -rate=$RATE/s -duration=$DURATION \
    -header="X-Forwarded-For: 192.168.1.1" \
    -header="X-Real-IP: 192.168.1.1" \
    -name=memory-test > "$RESULTS_DIR/vegeta-results-${TIMESTAMP}.bin" &
VEGETA_PID=$!

# Monitor memory every 30 seconds
echo ""
echo "Monitoring memory every 30 seconds..."
echo "Time,Heap_Bytes,Heap_MB,Heap_Objects,Stack_Bytes,Goroutines" > "$RESULTS_DIR/memory-timeseries-${TIMESTAMP}.csv"

ELAPSED=0
while kill -0 $VEGETA_PID 2>/dev/null; do
  HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
  HEAP_MB=$((HEAP / 1024 / 1024))
  OBJS=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_objects" | awk '{print $2}')
  STACK=$(curl -s "$METRICS_URL" | grep "^go_memstats_stack_inuse_bytes" | awk '{print $2}')
  GOROS=$(curl -s "$METRICS_URL" | grep "^go_goroutines" | awk '{print $2}')

  echo "${ELAPSED}s,${HEAP},${HEAP_MB},${OBJS},${STACK},${GOROS}" >> "$RESULTS_DIR/memory-timeseries-${TIMESTAMP}.csv"
  echo "[${ELAPSED}s] Heap: ${HEAP_MB} MB, Objects: ${OBJS}, Goroutines: ${GOROS}"

  sleep 30
  ELAPSED=$((ELAPSED + 30))
done

wait $VEGETA_PID
VEGETA_EXIT=$?

# Get final memory
echo ""
echo "Getting final memory..."
curl -s "$METRICS_URL" | grep "^go_memstats" | sort > "$RESULTS_DIR/metrics-final-${TIMESTAMP}.txt"
FINAL_HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
FINAL_HEAP_MB=$((FINAL_HEAP / 1024 / 1024))
echo "Final heap: $FINAL_HEAP bytes (${FINAL_HEAP_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Calculate growth
GROWTH=$((FINAL_HEAP - INITIAL_HEAP))
GROWTH_MB=$((GROWTH / 1024 / 1024))
echo "Heap growth: $GROWTH bytes (${GROWTH_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Report vegeta results
echo ""
echo "=== Vegeta Report ===" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
vegeta report "$RESULTS_DIR/vegeta-results-${TIMESTAMP}.bin" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

echo ""
echo "=== Test Summary ===" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
echo "Results saved to: $RESULTS_DIR/" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"

# Check if growth is acceptable
if [ $GROWTH_MB -lt 100 ]; then
  echo "✓ Memory growth is acceptable (<100 MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
  exit 0
else
  echo "✗ WARNING: Memory growth exceeded 100 MB (${GROWTH_MB} MB)" | tee -a "$RESULTS_DIR/test-${TIMESTAMP}.log"
  exit 1
fi
