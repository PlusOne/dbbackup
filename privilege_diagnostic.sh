#!/bin/bash
#
# Database Privilege Diagnostic Script
# Run this on both hosts to compare privilege states
#

echo "=============================================="
echo "Database Privilege Diagnostic Report"
echo "Host: $(hostname)"
echo "Date: $(date)"
echo "User: $(whoami)"
echo "=============================================="

echo ""
echo "1. DATABASE LIST WITH PRIVILEGES:"
echo "=================================="
sudo -u postgres psql -c "\l"

echo ""
echo "2. DATABASE PRIVILEGES (Detailed):"
echo "=================================="
sudo -u postgres psql -c "
SELECT 
    datname as database_name,
    datacl as access_privileges,
    datdba::regrole as owner
FROM pg_database 
WHERE datname NOT IN ('template0', 'template1')
ORDER BY datname;
"

echo ""
echo "3. ROLE/USER LIST:"
echo "=================="
sudo -u postgres psql -c "\du"

echo ""
echo "4. DATABASE-SPECIFIC GRANTS:"
echo "============================"
for db in $(sudo -u postgres psql -tAc "SELECT datname FROM pg_database WHERE datname NOT IN ('template0', 'template1', 'postgres')"); do
    echo "--- Database: $db ---"
    sudo -u postgres psql -d "$db" -c "
    SELECT 
        schemaname,
        tablename,
        tableowner,
        tablespace
    FROM pg_tables 
    WHERE schemaname = 'public'
    LIMIT 5;
    " 2>/dev/null || echo "Could not connect to $db"
done

echo ""
echo "5. GLOBAL OBJECT PRIVILEGES:"
echo "============================"
sudo -u postgres psql -c "
SELECT 
    rolname,
    rolsuper,
    rolcreaterole,
    rolcreatedb,
    rolcanlogin
FROM pg_roles
WHERE rolname NOT LIKE 'pg_%'
ORDER BY rolname;
"

echo ""
echo "6. CHECK globals.sql CONTENT (if exists):"
echo "========================================"
LATEST_CLUSTER=$(find /var/lib/pgsql/db_backups -name "cluster_*.tar.gz" -type f -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -1 | cut -d' ' -f2-)
if [ -n "$LATEST_CLUSTER" ]; then
    echo "Latest cluster backup: $LATEST_CLUSTER"
    TEMP_DIR="/tmp/privilege_check_$$"
    mkdir -p "$TEMP_DIR"
    tar -xzf "$LATEST_CLUSTER" -C "$TEMP_DIR" 2>/dev/null
    if [ -f "$TEMP_DIR/globals.sql" ]; then
        echo "globals.sql content:"
        echo "==================="
        head -50 "$TEMP_DIR/globals.sql"
        echo ""
        echo "... (showing first 50 lines, check full file if needed)"
        echo ""
        echo "Database creation commands in globals.sql:"
        grep -i "CREATE DATABASE\|GRANT.*DATABASE" "$TEMP_DIR/globals.sql" || echo "No database grants found"
    else
        echo "No globals.sql found in backup"
    fi
    rm -rf "$TEMP_DIR"
else
    echo "No cluster backup found to examine"
fi

echo ""
echo "=============================================="
echo "Diagnostic complete. Save this output and"
echo "compare between hosts to identify differences."
echo "=============================================="