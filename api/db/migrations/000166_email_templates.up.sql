-- Email templates (§6.8.1).
--
-- Cold email that works is short — 25 to 100 words, minimal HTML — because
-- heavy markup is both worse to read and worse for deliverability. So this is
-- a library of short texts with variables, not a drag-and-drop builder: the
-- editor everyone asks for produces exactly the emails that land in spam.

CREATE TABLE IF NOT EXISTS email_templates (
    id          CHAR(26)    PRIMARY KEY,
    account_id  CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    subject     TEXT        NOT NULL DEFAULT '',
    body        TEXT        NOT NULL DEFAULT '',

    -- Scope follows the same cascade as the brand kit (§6.7): a template can
    -- belong to the whole company, to one funnel, or to one salesperson.
    -- NULL funnel_id + NULL user_id = global.
    funnel_id   CHAR(26)    REFERENCES sales_funnels(id) ON DELETE CASCADE,
    user_id     CHAR(26)    REFERENCES users(id) ON DELETE CASCADE,

    -- Bumped on every save. The metrics below are per VERSION, because "this
    -- template gets 12% reply" is meaningless if the text changed twice since
    -- the number was earned.
    version     INTEGER     NOT NULL DEFAULT 1,

    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_templates_account
    ON email_templates (account_id, is_active);

-- Per template AND per version, for the reason above.
CREATE TABLE IF NOT EXISTS email_template_stats (
    template_id CHAR(26)    NOT NULL REFERENCES email_templates(id) ON DELETE CASCADE,
    version     INTEGER     NOT NULL,

    sent        INTEGER     NOT NULL DEFAULT 0,
    -- Opens are recorded and treated as a WEAK signal: Apple Mail Privacy
    -- Protection pre-fetches images, which inflates opens by roughly half.
    -- Kept because the trend still means something; never used alone.
    opened      INTEGER     NOT NULL DEFAULT 0,
    clicked     INTEGER     NOT NULL DEFAULT 0,
    replied     INTEGER     NOT NULL DEFAULT 0,
    bounced     INTEGER     NOT NULL DEFAULT 0,

    PRIMARY KEY (template_id, version)
);
