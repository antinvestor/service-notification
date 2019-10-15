
CREATE SEQUENCE languages_serial MINVALUE 0010100;

INSERT INTO languages(language_id,name, description,region, applied_at,created_at,modified_at,version) VALUES 
	(nextval('serial'),'English', 'This will be langusge for notifying client using emailing in english','England', now(),now(),now(),0);
