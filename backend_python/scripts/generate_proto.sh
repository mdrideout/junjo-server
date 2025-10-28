#!/bin/bash
# Generate Python code from Protocol Buffer definitions
# This script uses grpc_tools.protoc to generate both message classes and gRPC stubs

set -e  # Exit on error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROTO_DIR="$(cd "$PROJECT_ROOT/../proto" && pwd)"

echo "Generating Python protobuf code..."
echo "  Proto source: $PROTO_DIR"
echo "  Output directory: $PROJECT_ROOT/proto_gen"

# Generate Python code from proto files
uv run python -m grpc_tools.protoc \
  -I"$PROTO_DIR" \
  --python_out="$PROJECT_ROOT/proto_gen" \
  --grpc_python_out="$PROJECT_ROOT/proto_gen" \
  "$PROTO_DIR"/*.proto

# Fix imports to use relative imports (required for Python package imports)
for file in "$PROJECT_ROOT/proto_gen"/*_grpc.py; do
  if [ -f "$file" ]; then
    sed -i '' 's/^import \([a-z_]*\)_pb2 as/from . import \1_pb2 as/g' "$file"
  fi
done

echo "✓ Proto generation complete!"
echo "  Generated files:"
ls -1 "$PROJECT_ROOT/proto_gen"/*.py 2>/dev/null | xargs -n 1 basename || echo "  (no files generated)"
