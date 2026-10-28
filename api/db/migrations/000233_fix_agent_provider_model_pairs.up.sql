-- Four of six production agents carried provider='google' with model='gpt-4o'
-- — an OpenAI model name on a Google provider, a pairing that cannot answer
-- anything. Nothing validated it on save and nothing surfaced it on screen;
-- the account owner found out when the agent went quiet.
--
-- The cause was two INDEPENDENT defaults in CreateAgentUseCase (provider
-- defaulted to openai, model defaulted to gpt-4o), so an agent created as
-- google without naming a model inherited an OpenAI one. The default now
-- follows the provider, and domain.ValidateModelForProvider refuses the
-- pairing on create and on update. This repairs the rows that got in first.
--
-- Only the impossible direction is touched: a Google agent on a non-gemini
-- model. An OpenAI agent on an OpenAI model, or any model name this file has
-- never heard of, is left exactly as it is.
UPDATE agents
   SET model = 'gemini-3.7-flash',
       updated_at = NOW()
 WHERE provider IN ('google', 'gemini')
   AND model NOT LIKE 'gemini%';

UPDATE agents
   SET model = 'gpt-4o',
       updated_at = NOW()
 WHERE provider = 'openai'
   AND model LIKE 'gemini%';
