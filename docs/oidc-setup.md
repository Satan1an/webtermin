# OIDC SSO setup

webtermin speaks standard OpenID Connect with the Authorization-Code flow.
It's been tested against Authentik, Authelia, Keycloak, and Auth0, and should
work with anything that exposes a compliant `.well-known/openid-configuration`
endpoint.

This guide uses **Authentik** as the concrete example because it's the most
common self-hosted IdP. The shape is the same for everything else — only the
admin-console paths differ.

## Step 1 — register webtermin in your IdP

In Authentik **Applications → Providers → Create OAuth2/OpenID Provider**:

| Field                     | Value |
|---------------------------|-------|
| Name                      | `webtermin` |
| Authorization flow        | `default-authentication-flow` |
| Client type               | `Confidential` |
| Client ID                 | (autogen, copy this) |
| Client secret             | (autogen, copy this) |
| Redirect URIs             | `https://<your-host>:8443/api/auth/oidc/callback` |
| Scopes                    | `openid email profile` |
| Subject mode              | `Based on the user's username` (or hashed UID — either works) |

Then **Applications → Applications → Create** and point it at the provider
you just made.

If you self-host Authentik at `https://auth.example.com/`, the OIDC issuer URL
will be:

```
https://auth.example.com/application/o/webtermin/
```

(Find the exact URL on the provider's detail page under "OpenID Configuration
URL", minus the `/.well-known/openid-configuration` suffix.)

## Step 2 — fill in the webtermin config

Open `/etc/webtermin/config.yaml` and add an `oidc:` block:

```yaml
oidc:
  issuer: https://auth.example.com/application/o/webtermin/
  client_id: <copied from authentik>
  client_secret: <copied from authentik>
  redirect_url: https://<your-host>:8443/api/auth/oidc/callback
  default_role: viewer
```

`default_role` controls what role first-time SSO sign-ins get. Existing
panel users keep their already-stored role. Valid values: `viewer`,
`operator`, `admin`.

## Step 3 — restart and verify

```bash
sudo systemctl restart webtermin
sudo journalctl -u webtermin -n 20 --no-pager
```

You should see one of:

```
INFO oidc ready issuer=https://auth.example.com/application/o/webtermin/
```

…or, if discovery failed:

```
WARN oidc disabled: discovery failed err="…" issuer="…"
```

If discovery fails, local password login keeps working — the panel never
locks you out.

## Step 4 — try it

Open the login page in a private window. You should see a new button
below the password form:

> Sign in with SSO

Click it → IdP redirect → after consent → back to webtermin, signed in.

The first SSO sign-in creates a local account keyed on the IdP's `sub`
claim, with `preferred_username` (or `name`, or a sanitised subject) as
the username. Subsequent sign-ins reuse that account.

## Promoting an SSO user to admin

The default role is `viewer`. To grant more, sign in once as the local
admin (password) and then go to **Panel users → ⋯ → Change role**.

## Disabling local password login

webtermin keeps password login on as a recovery path even when OIDC is
configured. If you want OIDC-only, after promoting at least one SSO user
to admin:

```bash
# delete every panel user that wasn't created by OIDC, leaving the SSO
# admin in place. Easiest from the SQLite shell on the server:
sudo sqlite3 /var/lib/webtermin/webtermin.db \
  "DELETE FROM users WHERE username NOT LIKE 'oidc-%' AND username != 'your-sso-admin';"
sudo systemctl restart webtermin
```

(Doing this via the panel UI works too — Panel users → delete each
non-SSO row. The "last admin" guard ensures you can't leave zero
admins.)

## Troubleshooting

* **`oidc id_token verify: nonce did not match` or `signature is invalid`** —
  the `client_id` in webtermin's config doesn't match the one Authentik
  issued. ID tokens are verified against the configured `client_id`.

* **`oidc exchange: bad request` immediately after Authentik consent** —
  redirect URI mismatch. The URI in the IdP provider config must match
  `redirect_url` in webtermin's config *exactly*, including scheme, port,
  and path. `https://host:8443/api/auth/oidc/callback`, no trailing slash.

* **`state mismatch — possible CSRF, retry login`** — the short-lived state
  cookie expired (10 min). Just hit the button again. If it keeps
  happening, check that your reverse proxy isn't stripping cookies on the
  OAuth round-trip.

* **SSO works but the user has no permissions** — `default_role` is
  `viewer` unless you set otherwise. Promote them in Panel users.
