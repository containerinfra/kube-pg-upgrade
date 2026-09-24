-- Expected after upgrade: three known rows.
SELECT id, name, note FROM e2e_items ORDER BY id;
