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

DROP TABLE IF EXISTS auth_user CASCADE;
CREATE TABLE auth_user (
    id serial primary key,
    email character varying(254) NOT NULL,
    first_name character varying(150),
    last_name character varying(150),
    is_active boolean NOT NULL,
    is_staff boolean NOT NULL
);

DROP TABLE IF EXISTS flows_flow CASCADE;
CREATE TABLE flows_flow (id serial primary key);

DROP TABLE IF EXISTS tickets_ticket CASCADE;
CREATE TABLE tickets_ticket (id serial primary key);

DROP TABLE IF EXISTS msgs_broadcast CASCADE;
CREATE TABLE msgs_broadcast (id serial primary key);

DROP TABLE IF EXISTS msgs_optin CASCADE;
CREATE TABLE msgs_optin (id serial primary key);

DROP TABLE IF EXISTS msgs_msg CASCADE;
CREATE TABLE msgs_msg (
    id bigserial PRIMARY KEY,
    uuid uuid NOT NULL,
    org_id integer NOT NULL REFERENCES orgs_org(id) ON DELETE CASCADE,
    channel_id integer REFERENCES channels_channel(id) ON DELETE CASCADE,
    contact_id integer NOT NULL REFERENCES contacts_contact(id) ON DELETE CASCADE,
    contact_urn_id integer REFERENCES contacts_contacturn(id) ON DELETE CASCADE,
    broadcast_id integer REFERENCES msgs_broadcast(id) ON DELETE CASCADE,
    flow_id integer REFERENCES flows_flow(id) ON DELETE CASCADE,
    ticket_id integer REFERENCES tickets_ticket(id) ON DELETE CASCADE,
    created_by_id integer REFERENCES auth_user(id) ON DELETE CASCADE,
    text text NOT NULL,
    attachments character varying(255)[] NULL,
    quick_replies character varying(64)[] NULL,
    optin_id integer REFERENCES msgs_optin(id) ON DELETE CASCADE,
    locale character varying(6) NULL,
    created_on timestamp with time zone NOT NULL,
    modified_on timestamp with time zone NOT NULL,
    sent_on timestamp with time zone,
    msg_type character varying(1) NOT NULL,
    direction character varying(1) NOT NULL,
    status character varying(1) NOT NULL,
    visibility character varying(1) NOT NULL,
    msg_count integer NOT NULL,
    high_priority boolean NULL,
    error_count integer NOT NULL,
    next_attempt timestamp with time zone NOT NULL,
    failed_reason character varying(1),
    external_id character varying(255),
    metadata text,
    log_uuids uuid[]
);