#!/bin/bash

# First, drop the existing schema
~/go/bin/defradb client schema drop Block
~/go/bin/defradb client schema drop Transaction
~/go/bin/defradb client schema drop Event

# Apply the new schema
~/go/bin/defradb client schema add -f schema/schema.graphql
