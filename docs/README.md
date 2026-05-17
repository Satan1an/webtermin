# webtermin docs

| File                          | What |
|-------------------------------|------|
| [modules.md](modules.md)      | The 13 modules: what each does, which role can read / write, audit-log namespaces. |
| [api.md](api.md)              | Complete HTTP API reference grouped by module. The reference for anyone scripting against webtermin via API tokens. |
| [oidc-setup.md](oidc-setup.md)            | Concrete walkthrough of wiring up OIDC SSO, with Authentik as the worked example. |
| [non-root-setup.md](non-root-setup.md)    | How to run the panel as an unprivileged user via sudoers + the docker group. |
| [contributing.md](contributing.md)        | Repo layout, build / test / lint, the established pattern for adding a new module. |
| [images/](images/)            | Screenshots used by the project README. See `images/README.md` for the recommended capture / replace flow. |

For the high-level overview see the top-level [README.md](../README.md)
(also available in [Russian](../README.ru.md)). For the security
policy and the v0.1 pre-release audit summary see
[SECURITY.md](../SECURITY.md).
