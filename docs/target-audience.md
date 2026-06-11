# Target Audience

TPT Online Video is built for **people who want to own their video platform** — not for billion-dollar corporations running global CDNs.

## Who This Is For

### Small Communities

- **Gaming communities** that want to host their own highlight reels, tutorial archives, or tournament VODs without uploading to YouTube's algorithm
- **Open-source project teams** running their own conference talk archives, screencast libraries, or documentation videos alongside their code
- **Hobbyist groups** (music, dance, sports, creative arts) that want a shared video space without ads, tracking, or content-ID takedowns
- **Educational collectives** (co-ops, study groups, open courseware) hosting lectures and tutorials on their own infra
- **Religious or cultural organisations** that need control over their content and who can access it

### Individuals

- **Solo creators** who want full ownership of their content and don't want to be at the mercy of platform policies, demonetisation, or algorithm changes
- **Privacy-conscious users** who don't trust big tech with their viewing habits or upload history
- **Hobbyist sysadmins** who enjoy running their own infrastructure and learning about distributed systems, transcoding pipelines, and CDN design
- **Developers** who want a hackable video platform they can extend, customise, and learn from
- **Tech-savvy families** running a private video archive for home movies, events, and memories

### Self-Hosters & De-Centralisation Advocates

- People who run a home server (or $6/month VPS) and want a YouTube-like experience that's **theirs**
- Communities that have been burned by platform shutdowns (Google Reader, Vine, etc.) and want their content on infrastructure they control
- Advocates for the **small internet** — the idea that anyone should be able to run a service for their friends, not just companies with data centres

## Who This Is NOT For

TPT Online Video is **not** designed for:

- **YouTube-scale** — If you need 10,000+ concurrent uploaders or petabyte-scale storage, you're better served by custom infrastructure or a commercial platform
- **Global CDN distribution** — No built-in edge node replication. You can front it with Cloudflare or a CDN, but that's on you
- **Enterprise SLA guarantees** — This is MIT open-source software. No warranties, no uptime promises, no support contract
- **ActivityPub federation** (yet) — Currently a single-instance platform. Cross-instance communication is not implemented

## Why Self-Host?

| | YouTube / Vimeo | TPT Online Video |
|---|---|---|
| **Cost** | Free with ads, or $10-30/mo | Your VPS or hardware cost |
| **Control** | None — your content, their rules | Full — you control everything |
| **Privacy** | You are the product | Zero tracking, no analytics |
| **Content policy** | Subject to arbitrary moderation | You decide what's allowed |
| **Ads** | You don't get paid, they do | None |
| **Algorithm** | Optimised for engagement, not you | No algorithm — browse what you want |
| **Longevity** | Your channel can be terminated | Runs as long as your server does |
| **Customisation** | Limited | Full source code access, MIT licensed |

## The Philosophy

> **Anyone should be able to host a video platform for their friends.**

Not just companies. Not just people who can afford enterprise infrastructure. A person with a $10/month VPS and a weekend should be able to set up a place where their community can upload, watch, and share videos. No ads. No tracking. No algorithm. No one telling you your content doesn't fit their corporate guidelines.

TPT Online Video exists to make that possible.

## Community Examples

The kinds of communities we envision running TPT Online Video:

| Group Size | Example | Hosting |
|---|---|---|
| 5-20 people | A gaming clan sharing highlights | $6/mo VPS |
| 20-100 people | An open-source project's screencast archive | $12/mo VPS |
| 100-500 people | A university club's lecture recordings | $24/mo VPS + storage |
| 500-5000 people | A music scene's live performance archive | Dedicated server or cloud |

These are estimates — actual capacity depends on concurrent uploads, transcoding load, and storage requirements.

## See Also

- [Deployment options](./deployment.md) — how to get it running on various platforms
- [Architecture](./architecture.md) — how the system is designed for single-node deployment
- [Deployment guide for Windows desktop](./deployment/windows-desktop.md) — running on your own machine