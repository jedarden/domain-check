#!/bin/bash
# Workflow Entrypoint Test Script
# Usage: ./run-workflow-tests.sh
# Prerequisites: Valid iad-ci credentials in ~/.kube/iad-ci.kubeconfig

set -e

KUBECONFIG="${KUBECONFIG:-/home/coding/.kube/iad-ci.kubeconfig}"
NAMESPACE="argo-workflows"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "========================================="
echo "Workflow Entrypoint Test"
echo "========================================="
echo "Kubeconfig: $KUBECONFIG"
echo "Namespace: $NAMESPACE"
echo ""

# Check credentials
echo "Checking credentials..."
if ! kubectl --kubeconfig="$KUBECONFIG" get workflows -n "$NAMESPACE" --request-timeout=5s &>/dev/null; then
    echo "❌ CREDENTIAL ERROR: Cannot access iad-ci cluster"
    echo "The ServiceAccount token has expired. Regenerate from Rackspace Spot UI:"
    echo "1. Login to Rackspace Spot"
    echo "2. Navigate to iad-ci cloudspace"
    echo "3. Get new OIDC token for cloudspace-admin"
    echo "4. Update ~/.kube/iad-ci.kubeconfig"
    exit 1
fi
echo "✓ Credentials valid"
echo ""

# Test 1: Build workflow (default entrypoint)
echo "========================================="
echo "Test 1: Build Workflow (Default Entry Point)"
echo "========================================="
echo "Expected: Should run build-quality-gate → resolve-version → docker-build"
echo "Should NOT run goreleaser-release"
echo ""

BUILD_OUTPUT=$(kubectl --kubeconfig="$KUBECONFIG" create -f "$SCRIPT_DIR/workflow-build-test.yaml" -n "$NAMESPACE" -o json)
BUILD_NAME=$(echo "$BUILD_OUTPUT" | jq -r '.metadata.name')

echo "✓ Workflow submitted: $BUILD_NAME"
echo ""
echo "Monitoring workflow (press Ctrl+C to stop monitoring)..."
kubectl --kubeconfig="$KUBECONFIG" wait --for=condition=Succeeded --for=condition=Failed "workflow/$BUILD_NAME" -n "$NAMESPACE" --timeout=45m || true

echo ""
echo "Fetching workflow status..."
kubectl --kubeconfig="$KUBECONFIG" get workflow "$BUILD_NAME" -n "$NAMESPACE"

echo ""
echo "Step execution graph:"
kubectl --kubeconfig="$KUBECONFIG" get workflow "$BUILD_NAME" -n "$NAMESPACE" -o json | \
  python3 -c "
import json, sys
w = json.load(sys.stdin)
for node in w['status'].get('nodes', {}).values():
    if 'workflow' not in node.get('type', ''):
        print(f\"  {node['displayName']:30s} {node.get('phase', 'Unknown'):10s} {node.get('message', '')[:50]}\")
"

echo ""
echo "Verifying NO goreleaser step ran..."
if kubectl --kubeconfig="$KUBECONFIG" get workflow "$BUILD_NAME" -n "$NAMESPACE" -o json | \
  python3 -c "
import json, sys
w = json.load(sys.stdin)
nodes = [n for n in w['status'].get('nodes', {}).values() if 'goreleaser' in n.get('displayName', '').lower()]
sys.exit(0 if len(nodes) == 0 else 1)
"; then
    echo "✓ PASS: No goreleaser step in build workflow"
else
    echo "❌ FAIL: goreleaser step found in build workflow (should not exist)"
fi

echo ""
echo "Press Enter to continue to Test 2..."
read

# Test 2: Release workflow (explicit entrypoint)
echo ""
echo "========================================="
echo "Test 2: Release Workflow (Explicit Entry Point)"
echo "========================================="
echo "Expected: Should run quality-gate → goreleaser-release"
echo "Expected failure at goreleaser (v0.0.0-test tag doesn't exist)"
echo "Should NOT run build-quality-gate, resolve-version, or docker-build"
echo ""

RELEASE_OUTPUT=$(kubectl --kubeconfig="$KUBECONFIG" create -f "$SCRIPT_DIR/workflow-release-test.yaml" -n "$NAMESPACE" -o json)
RELEASE_NAME=$(echo "$RELEASE_OUTPUT" | jq -r '.metadata.name')

echo "✓ Workflow submitted: $RELEASE_NAME"
echo ""
echo "Monitoring workflow (press Ctrl+C to stop monitoring)..."
kubectl --kubeconfig="$KUBECONFIG" wait --for=condition=Succeeded --for=condition=Failed "workflow/$RELEASE_NAME" -n "$NAMESPACE" --timeout=45m || true

echo ""
echo "Fetching workflow status..."
kubectl --kubeconfig="$KUBECONFIG" get workflow "$RELEASE_NAME" -n "$NAMESPACE"

echo ""
echo "Step execution graph:"
kubectl --kubeconfig="$KUBECONFIG" get workflow "$RELEASE_NAME" -n "$NAMESPACE" -o json | \
  python3 -c "
import json, sys
w = json.load(sys.stdin)
for node in w['status'].get('nodes', {}).values():
    if 'workflow' not in node.get('type', ''):
        print(f\"  {node['displayName']:30s} {node.get('phase', 'Unknown'):10s} {node.get('message', '')[:50]}\")
"

echo ""
echo "Verifying goreleaser step was ATTEMPTED..."
if kubectl --kubeconfig="$KUBECONFIG" get workflow "$RELEASE_NAME" -n "$NAMESPACE" -o json | \
  python3 -c "
import json, sys
w = json.load(sys.stdin)
nodes = [n for n in w['status'].get('nodes', {}).values() if 'goreleaser' in n.get('displayName', '').lower()]
sys.exit(1 if len(nodes) > 0 else 0)
"; then
    echo "❌ FAIL: No goreleaser step found (entrypoint routing may not work)"
else
    echo "✓ PASS: goreleaser step reached (entrypoint routing works)"
fi

echo ""
echo "Verifying NO build-quality-gate step ran..."
if kubectl --kubeconfig="$KUBECONFIG" get workflow "$RELEASE_NAME" -n "$NAMESPACE" -o json | \
  python3 -c "
import json, sys
w = json.load(sys.stdin)
nodes = [n for n in w['status'].get('nodes', {}).values() if 'build-quality-gate' in n.get('displayName', '').lower()]
sys.exit(0 if len(nodes) == 0 else 1)
"; then
    echo "✓ PASS: No build-quality-gate step in release workflow"
else
    echo "❌ FAIL: build-quality-gate step found in release workflow (should not exist)"
fi

echo ""
echo "Verifying workflow FAILED at goreleaser (expected)..."
PHASE=$(kubectl --kubeconfig="$KUBECONFIG" get workflow "$RELEASE_NAME" -n "$NAMESPACE" -o jsonpath='{.status.phase}')
if [ "$PHASE" = "Failed" ]; then
    echo "✓ PASS: Workflow failed as expected (tag v0.0.0-test doesn't exist)"
else
    echo "❌ FAIL: Workflow should have failed but got phase: $PHASE"
fi

echo ""
echo "========================================="
echo "Test Complete"
echo "========================================="
echo ""
echo "Workflow run IDs:"
echo "  Build:  $BUILD_NAME"
echo "  Release: $RELEASE_NAME"
echo ""
echo "View in Argo UI: https://argo-ci.ardenone.com"
echo ""
