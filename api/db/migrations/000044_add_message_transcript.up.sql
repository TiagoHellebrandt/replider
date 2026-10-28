-- Cache the Whisper/Gemini transcription output on the message row
-- itself so the dashboard can re-render the transcript without paying
-- the 5-10s transcription latency (and the OpenAI $ per minute) on
-- every page reload. NULL = not yet transcribed. Empty string is a
-- valid "transcriber returned nothing" sentinel.
ALTER TABLE messages ADD COLUMN transcript TEXT;
