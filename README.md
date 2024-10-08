# streaming-txn-status

a proof of concept to show how to get realtime updates using LISTEN and NOTIFY functionality provided by postgres.


When transactions were created
![](./images/2024-09-25-08-57-04.png)

When status of id=21 is changed
![](./images/2024-09-25-08-57-43.png)

As status is changing for a given transaction id, the socket is able to receive updates
![](./images/2024-09-25-08-58-22.png)

Also received signal for more than one update
![](./images/2024-09-25-08-59-02.png)



## How to run
- Webserver: `go run server/main.go`
- Client: `go run client/main.go $id` where id = transaction id


## How to setup database
```sql
-- Step 1: Create the database
-- Run this command in your PostgreSQL shell or using a database client
DROP DATABASE test_bench;

CREATE DATABASE test_bench;

-- Step 2: Connect to the database
-- In psql shell, run:
\c test_bench 
-- Step 3: Create the 'transactions' table
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    status VARCHAR(50),
    amount INT
);

-- Step 4: Insert some initial data
INSERT INTO transactions VALUES (23, 'pending', 200);

insert into transactions VALUES (24, 'pending', 300);

insert into transactions VALUES (25, 'pending', 500);

-- Step 5: Create the trigger function
CREATE OR REPLACE FUNCTION notify_transaction_status_change()
RETURNS trigger AS $$
DECLARE
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.status IS DISTINCT FROM NEW.status THEN
        PERFORM pg_notify('transaction_status_changed', NEW.id::text || ',' || NEW.status);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Step 6: Create the trigger
CREATE TRIGGER transaction_status_change_trigger AFTER
UPDATE ON transactions FOR EACH ROW
EXECUTE PROCEDURE notify_transaction_status_change ();

```
