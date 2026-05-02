#!/bin/bash

# Configuration
DB_URL="postgres://synthify:synthify@localhost:5432/synthify?sslmode=disable"
OUTPUT_FILE="explain_results.txt"

echo "Database Query EXPLAIN Results (Fixed Quotes)" > $OUTPUT_FILE
echo "Generated on: $(date)" >> $OUTPUT_FILE
echo "==============================" >> $OUTPUT_FILE

# Process each .sql file in db/queries
for file in db/queries/*.sql; do
    echo "Processing $file..."
    echo "" >> $OUTPUT_FILE
    echo "--- File: $file ---" >> $OUTPUT_FILE
    
    # Use perl to extract queries and keep them on a single line
    # Use single quotes for dummy values - escaped correctly for shell/perl
    perl -ne '
        if (/^-- name:\s+(\w+)/) {
            if ($query) { 
                $query =~ s/\$[0-9]+/'\''dummy'\''/g;
                $query =~ s/sqlc\.arg\(\w+\)/'\''dummy'\''/g;
                $query =~ s/sqlc\.narg\(\w+\)/'\''dummy'\''/g;
                print "EXPLAIN $query;\n"; 
            }
            $query = "";
        } elsif (/^--/) {
            next;
        } elsif (/\S/) {
            chomp;
            $query .= " $_";
        }
        END { 
            if ($query) { 
                $query =~ s/\$[0-9]+/'\''dummy'\''/g;
                $query =~ s/sqlc\.arg\(\w+\)/'\''dummy'\''/g;
                $query =~ s/sqlc\.narg\(\w+\)/'\''dummy'\''/g;
                print "EXPLAIN $query;\n"; 
            } 
        }
    ' "$file" > temp_queries.sql

    # Execute each EXPLAIN query
    while IFS= read -r line; do
        if [[ -z "$line" ]]; then continue; fi
        
        # Strip potential trailing semicolons from inner queries to avoid ;;
        clean_line=$(echo "$line" | sed 's/;[[:space:]]*$//')
        full_command="${clean_line};"
        
        echo "Running: $full_command"
        echo "Query: $full_command" >> $OUTPUT_FILE
        psql "$DB_URL" -c "$full_command" >> $OUTPUT_FILE 2>&1
        echo "------------------------------" >> $OUTPUT_FILE
    done < temp_queries.sql
done

rm temp_queries.sql
echo "Done. Results saved to $OUTPUT_FILE"
