DROP TABLE IF EXISTS orgs_org CASCADE;
CREATE TABLE orgs_org (
    id serial primary key,
    name character varying(255) NOT NULL,
    is_active boolean NOT NULL
);

DROP TABLE IF EXISTS channels_channel CASCADE;
CREATE TABLE channels_channel (
    id serial primary key,
    uuid character varying(36) NOT NULL,
    org_id integer REFERENCES orgs_org(id) ON DELETE CASCADE,
    is_active boolean NOT NULL,
    channel_type character varying(3) NOT NULL,
    name character varying(64),
    schemes character varying(16)[] NOT NULL,
    address character varying(64),
    country character varying(2),
    config jsonb NOT NULL,
    role character varying(4) NOT NULL,
    log_policy character varying(1) NOT NULL,
    created_on timestamp with time zone NOT NULL,
    modified_on timestamp with time zone NOT NULL,
    created_by_id integer NOT NULL,
    modified_by_id integer NOT NULL
);

DROP TABLE IF EXISTS contacts_contact CASCADE;
CREATE TABLE contacts_contact (
    id serial primary key,
    uuid character varying(36) NOT NULL,
    org_id integer references orgs_org(id) ON DELETE CASCADE,
    is_active boolean NOT NULL,
    status character varying(1) NOT NULL,
    ticket_count integer NOT NULL,
    created_on timestamp with time zone NOT NULL,
    modified_on timestamp with time zone NOT NULL,
    name character varying(128),
    language character varying(3)
);

DROP TABLE IF EXISTS contacts_contacturn CASCADE;
CREATE TABLE contacts_contacturn (
    id serial primary key,
    identity character varying(255) NOT NULL,
    path character varying(255) NOT NULL,
    scheme character varying(128) NOT NULL,
    display character varying(128) NULL,
    priority integer NOT NULL,
    channel_id integer references channels_channel(id) on delete cascade,
    contact_id integer references contacts_contact(id) on delete cascade,
    org_id integer NOT NULL references orgs_org(id) on delete cascade,
    auth_tokens jsonb,
    UNIQUE (org_id, identity)
);