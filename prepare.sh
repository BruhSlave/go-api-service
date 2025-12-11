#!/bin/bash

POSTGRES_USER=validator
POSTGRES_DB=project-sem-1

psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f prices_table.sql

