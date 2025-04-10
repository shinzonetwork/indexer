#!/bin/bash

# Start Defra with rootdir
~/go/bin/defradb start --rootdir ~/Developer/shinzo/version1/.defra & 

# Apply the new schema
~/go/bin/defradb client schema add -f schema/schema.graphql
