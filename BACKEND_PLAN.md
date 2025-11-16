Backend

DB
* PG 18 DB.
* Replicates schema of SQLite to start, PLUS
    * Sessions always owned by a user concept.  Can fill with user=default for now.

Backend
* Golang
* APIs:
    * Support “confab save”:
        * Metadata: Upserting sessions, runs, files records to DB
        * Data: File contents of all referenced JSONL files.  Store in object store (S3).
            * DB schema should link files => S3 keys.

For local dev, have docker-compose setup for pg 18, and minio (S3).

Auth
* Need a simple modern auth system.
* we will have a users table.
* also need an API key or token system, especially for `confab save`.

Payments
* This can be added on later.  Start with free service.

