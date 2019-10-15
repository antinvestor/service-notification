
CREATE SEQUENCE serial MINVALUE 00101;


INSERT INTO Channels(Channels_id,Channel, description, applied_at,created_at,modified_at,version) VALUES 
	(nextval('serial'),'Email', 'This will be channel for notifying client using emailin', now(),now(),now(),0);



	