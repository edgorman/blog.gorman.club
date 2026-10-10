# 12. The assistant is a subscription, rate limited per account

Status: Accepted

## Context

The assistant is the one feature that calls a paid model on every turn, so who may use it and how much both need bounding.

## Decision

- Access is a whitelist in the access model ([13](0013-one-access-policy-table.md)) whose membership is worked out per request: an account is entitled while `subscribedUntil` on its profile has not passed, and no other way. There is no per-environment allowlist. Granting is writing the field, revoking is clearing it, and neither needs a deploy. It is set by hand until a checkout writes it.
- The subscription follows the account (the uid in the verified ID token), never an address or a username, which is freely chosen and claimable by anybody once released.
- Turns are rate limited per account on a much tighter budget than any other route. The buckets live in the serving process.

## Consequences

The rate limit is a per-instance budget. Revisit it, along with the feed cache ([15](0015-cache-only-the-anonymous-feed.md)), if the service ever scales past one instance.
