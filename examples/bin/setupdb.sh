#!/bin/bash

export PGPASSWORD=pass
psql -U postgres -d postgres -p 5433 -h localhost < database/migrations/up.sql