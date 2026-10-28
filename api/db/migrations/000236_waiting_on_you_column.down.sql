-- Removes only the columns this migration could have created, and only while
-- they are empty: a stage holding cards cannot be dropped without deciding
-- where those cards go, and that is not a decision a rollback should make.
DELETE FROM funnel_stages s
 WHERE s.name = 'Esperando você'
   AND s.maps_to_status = 'pending_approval'
   AND NOT EXISTS (SELECT 1 FROM leads l WHERE l.stage_id = s.id);
