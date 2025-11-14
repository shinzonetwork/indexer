#!/bin/bash

# Apply the new schema
~/go/bin/defradb client schema add -f pkg/schema/schema.graphql
