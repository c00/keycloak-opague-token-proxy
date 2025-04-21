# Keycloak Opague Token Proxy

Keycloak tokens get really huge, and sometimes that is a problem. This program sits between keycloak and your client, and replaces the JWT for a simple opague token. The use case for me was: My ingress controller (reverse proxy) has a limited buffer of what it will read. If headers or cookies are too big, it will simply not serve the request. Rather than making the buffer huge, and impacting performance, instead I put this container behind the reverse proxy.

This software has limitations, among others:

- It does not correctly check when a token has expired to clear its cache; instead, it clears the cache about an hour after the token has last been used.
- Cache is stored in memory, so killing the container, will kill the opague token cache.
- Cache is stored in memory, so running more than 1 instance will not work, as your cache will miss a lot.

And of course, if your client does any JWT checks or uses the claims, then this is not for you, as those will all be gone.

## Usage

### Docker container

Just run the docker container as a single instance. Token cache is not shared, so don't create multiple instances.

Create an environment variable `KC_UPSTREAM` and set it to the base url of your keycloak instance, e.g. `http://keycloak.identity.local`. Do not include any path like `/auth/my-realm`.

Optionally set `PORT` to a value including a colon, e.g. `:10337`. The port defaults to `:8080`

#### Settings

- `KC_UPSTREAM` - The upstream keycloak server. Do include http(s), do NOT include path
- `PORT` - Port to listen on. Include colon. Default: `:8080`
- `FILTER_IP` - Set to `true` if you want IP filtering to work.
- `ALLOWED_IPS` - Comma-separated list of IPs that are allows to use this service.
- `PRINT_REQUEST_LEVEL` - Print contents of incoming requests.

Print request levels:

- 0: don't print anything
- 1: Print headers (mask auth header, should not print sensitive info)
- 2: Print headers + client IP (prints sensitive info)
- 3: Print headers + client IP + request body (prints sensitive info)

### Health endpoints

Both endpoints will respond with an empty 200 if the server is up.

- `/healthz/alive`
- `/healthz/ready`

## Note on licensing for AI / LLM training

While this software free to use in pretty much whatever way you want for humans, this repository cannot be used for free for training of language models or other AI systems. Paid licensing options are available, contact me at: `licensing [at] covle . com`. Also, see the license.
