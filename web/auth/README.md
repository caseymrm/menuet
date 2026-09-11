# auth.menuet.app

Generic OAuth redirect trampoline for menuet apps, deployed to the
`menuet-auth` Cloudflare Pages project (custom domain `auth.menuet.app`).

Providers that require an https redirect URI get
`https://auth.menuet.app/<app>`; the page forwards the query string to the
app's loopback listener via a top-level https→http navigation, which
browsers allow for 127.0.0.1. Registering a new app is one entry in the
`apps` map in `index.html` plus that URI registered with the provider.

Deploy:

    npx wrangler pages deploy web/auth --project-name menuet-auth --branch main
