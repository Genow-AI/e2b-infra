import { Sandbox } from 'e2b'
import dotenv from 'dotenv'
dotenv.config()

// Self-hosted e2b: both are required. E2B_DOMAIN is your deployment's base domain
// (from DOMAIN_NAME in .env.gcp), NOT the public e2b.dev.
const domain = process.env.E2B_DOMAIN
const apiKey = process.env.E2B_API_KEY

if (!apiKey || !domain) {
  console.error('Set both E2B_API_KEY and E2B_DOMAIN (e.g. E2B_DOMAIN=e2b-sandbox.genow.cloud).')
  process.exit(1)
}

console.log(`Creating a sandbox on ${domain} ...`)
const sbx = await Sandbox.create({ apiKey, domain })
console.log('Sandbox created:', sbx.sandboxId)

const res = await sbx.commands.run('echo "hello from e2b" && uname -a')
console.log('--- output ---')
console.log(res.stdout.trim())
if (res.stderr) console.error('stderr:', res.stderr)

await sbx.kill()
console.log('Sandbox killed. ✅ hello-world complete.')
