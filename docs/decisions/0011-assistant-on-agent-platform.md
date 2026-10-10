# 11. The writing assistant calls Gemini on the Agent Platform, with no API key

Status: Accepted

## Context

Gemini models are reachable two ways: the Gemini API (`generativelanguage.googleapis.com`) and the Gemini Enterprise Agent Platform, formerly Vertex AI, still served at `aiplatform.googleapis.com`. The Gemini API's `generateContent` declares no OAuth scope, so it accepts an API key and nothing else; a token-authorized request gets a `403`. The platform's `generateContent` declares `cloud-platform`, so it accepts the credential Cloud Run already has.

## Decision

Call the Agent Platform as the Cloud Run runtime service account (`roles/aiplatform.user`) over Application Default Credentials. The model id and location are Terraform variables (`assistant_model`, `assistant_location`), because model availability is regional and model ids change faster than the service is redeployed. The worker's embedding and moderation models follow the same pattern.

## Consequences

There is no long-lived credential to store, rotate or leak, the same reasoning that put CI on Workload Identity Federation ([27](0027-workload-identity-federation.md)). Using the Gemini API instead would introduce exactly that secret.
