#!/bin/bash
# seed_users.sh - Register test users for k6 load test

TARGET_URL="https://backend-production-7e53.up.railway.app"
NUM_USERS=100
PASSWORD="Password123!"

echo "=== Seeding $NUM_USERS load test users to $TARGET_URL ==="

for i in $(seq 1 $NUM_USERS); do
  EMAIL="user${i}@loadtest.com"
  
  # Register user via API
  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$TARGET_URL/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
  
  if [ "$RESPONSE" -eq 201 ]; then
    echo "[$i/$NUM_USERS] Created user: $EMAIL"
  elif [ "$RESPONSE" -eq 409 ]; then
    echo "[$i/$NUM_USERS] User already exists: $EMAIL"
  else
    echo "[$i/$NUM_USERS] Failed to create user $EMAIL (HTTP $RESPONSE)"
  fi
done

echo "=== Seeding completed ==="
