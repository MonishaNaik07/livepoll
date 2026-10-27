# Demo video script (3–5 min). Record it yourself and speak in your own words.

The two questions the reviewers ask must be answered from YOUR real experience. Fill the blanks; don't read invented answers.

**0:00–0:30 Intro.** Name, project, stack (React, Go/Gin, MongoDB, Redis), live URL.
**0:30–2:15 Live demo (deployed URL).** Sign up → create poll → copy link → open in incognito → vote → show the first window updating with no refresh. Show a duplicate vote being rejected. Close the poll and show the status flip live.
**2:15–3:15 Architecture.** Vote path: validate → Mongo insert (unique `pollId+voterId`) → Redis Lua apply (idempotent) → Pub/Sub → every Go instance → WebSocket. Show a Mongo vote document and `HGETALL poll:<id>:counts` in Redis. Explain recovery: Mongo is source of truth; Redis rebuilt from it; reconciler replays unapplied votes.
**3:15–4:15 Hardest challenge.** `[YOUR real challenge + how you solved it. Be specific.]`
**4:15–4:50 AI usage.** `[Which tools you used, what they helped with, where they got in the way, what you had to fix yourself.]`
**4:50–5:00 Close.**

Reminder: you will be asked to explain every part of this in technical interviews. Read the code you submit.
