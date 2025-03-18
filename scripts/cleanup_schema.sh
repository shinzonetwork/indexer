#!/bin/bash

# Drop existing collections
defradb client schema migration drop Block
defradb client schema migration drop Transaction
defradb client schema migration drop Log
defradb client schema migration drop Event

# Apply new schema
./scripts/apply_schema.sh
