# threnslator
# LINE Translator Bot (Go)


Bi-directional EN↔TH translator for LINE groups and 1:1 chats. Joins a group and automatically translates every text message using OpenAI.


## Setup

1. In LINE Developers Console, under your Provider create a **Messaging API** channel. Enable "Receive messages" and set Webhook URL to `https://<your-domain>/line/webhook`. Verify webhook.
2. Issue a **Channel access token** (long-lived). Copy the **Channel secret**.
3. Create an OpenAI API key with access to GPT-3.5 Turbo.
4. (Optional) Set up Cloudflare Tunnel:
   - Log into the [Cloudflare Zero Trust Dashboard](https://one.dash.cloudflare.com)
   - Go to Access > Tunnels
   - Create a new tunnel and copy the token
   - The token will be used as the `TUNNEL_TOKEN` environment variable
5. `cp .env.example .env` and fill values.
6. Run locally:


```bash
export $(grep -v '^#' .env | xargs -0 -I {} echo {} | tr '\n' ' ')
go run .
```


### Using Docker

You can either build locally or use the pre-built image from GitHub Container Registry:

```bash
# Option 1: Build locally
docker build -t line-translator-bot .

# Option 2: Pull from GitHub Container Registry
docker pull ghcr.io/emanuele-g/line-translator-bot:latest

# Run with environment file (Recommended)
docker run --rm -p 8080:8080 --env-file .env ghcr.io/emanuele-g/line-translator-bot:latest

# Or run with environment variables
export $(grep -v '^#' .env | xargs -0 -I {} echo {} | tr '\n' ' ')
docker run --rm -p 8080:8080 \
  -e LINE_CHANNEL_SECRET \
  -e LINE_CHANNEL_TOKEN \
  -e OPENAI_API_KEY \
  -e TUNNEL_TOKEN \
  ghcr.io/emanuele-g/line-translator-bot:latest
```

The Docker image is automatically built and published to GitHub Container Registry on every push to main and when tags are created.


If not using Cloudflare Tunnel, expose `8080` on a public HTTPS endpoint. With Cloudflare Tunnel, the application will automatically be exposed through a secure tunnel. On first deploy, hit **Verify** in the LINE console.


## Behavior


• Text in Thai becomes English. Text in English becomes Thai with natural particles for a male speaker by default. No extra explanations.
• Replies include a short direction tag like `[en→th]` or `[th→en]` to reduce confusion in mixed-language threads.
• Works in groups and 1:1 chats.


## Notes


• Signature verification is handled by `bot.ParseRequest` from the official SDK.
• If you want the bot to translate only when mentioned, add a check like `strings.Contains(m.Text, "@yourbot")` before calling `translate`.
• To reduce cost/latency, you can add caching by message hash or use a different model.
• For enterprise use, consider rate limiting and a queue worker for long messages.