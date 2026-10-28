-- Configuration inheritance (§6.7): the company standardises, the salesperson
-- personalises, compliance never bends.
--
-- Every salesperson sells their own way, and a product that forces one voice
-- on all of them gets worked around. But a healthcare distributor cannot let
-- each seller invent claims about what a device does — that is a regulatory
-- problem, not a style preference.

CREATE TABLE IF NOT EXISTS brand_kits (
    account_id     CHAR(26)    PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,

    company_name   TEXT        NOT NULL DEFAULT '',
    about          TEXT        NOT NULL DEFAULT '',
    tone_of_voice  TEXT        NOT NULL DEFAULT '',

    logo_url       TEXT        NOT NULL DEFAULT '',
    primary_color  TEXT        NOT NULL DEFAULT '',
    signature      TEXT        NOT NULL DEFAULT '',

    -- What a salesperson MAY say, listed rather than implied so a new seller
    -- has a source instead of a guess.
    allowed_claims   TEXT[]    NOT NULL DEFAULT '{}',
    -- What nobody may say, at any level. In healthcare this is where a wrong
    -- sentence stops being a style problem and becomes a regulatory one.
    forbidden_claims TEXT[]    NOT NULL DEFAULT '{}',

    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One salesperson's personal layer. Additive by construction: there is
-- deliberately no column here that can weaken the company layer, because the
-- ability to personalise must not become an escape hatch from compliance.
CREATE TABLE IF NOT EXISTS seller_overlays (
    account_id  CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id     CHAR(26)    NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    tone_note   TEXT        NOT NULL DEFAULT '',
    -- The one thing that SHOULD be personal: it carries a name and a phone
    -- number.
    signature   TEXT        NOT NULL DEFAULT '',

    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, user_id)
);
