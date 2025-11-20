#!/bin/bash

# Purge the database
~/go/bin/defradb client purge --force

# Apply the new schema
~/go/bin/defradb client schema add -f pkg/schema/schema.graphql