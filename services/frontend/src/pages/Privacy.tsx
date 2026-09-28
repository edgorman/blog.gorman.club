import { Link } from 'react-router-dom'
import { PageMeta } from '../components/PageMeta'
import { GITHUB_REPO } from '../lib/format'
import { STORAGE_ITEMS } from '../lib/storage'

const OWNER_URL = 'https://github.com/edgorman'
const ISSUES_URL = `https://github.com/${GITHUB_REPO}/issues`

/** The privacy policy and cookie notice (#238). Static text: a change to what it describes changes this page too. */
export function Privacy() {
  return (
    <div className="page">
      <PageMeta
        title="Privacy policy"
        description="What the Gorman Club blog collects, why, who processes it, and what your browser stores."
        path="/privacy"
      />
      <header className="page-header">
        <span className="page-kicker text-muted">Legal</span>
        <h1 className="title-post">Privacy policy</h1>
        <p className="text-muted">Last updated 28 September 2026</p>
      </header>

      <div className="post-body">
        <h2>Who runs the site</h2>
        <p>
          blog.gorman.club is run by Edward Gorman, who is the controller of the personal data described here. To ask
          about it or exercise your rights, contact him through <a href={OWNER_URL}>GitHub</a>. Please don't put
          personal details in a public <a href={ISSUES_URL}>issue</a>.
        </p>

        <h2>What is collected</h2>
        <ul>
          <li>
            <strong>Your Google account details.</strong> Signing in gives the site a token from Google with your
            account ID, email address, name and whether the email is verified. Only the account ID is stored, as your
            profile's ID; the rest is used for the request and not saved.
          </li>
          <li>
            <strong>Your profile:</strong> your username, bio, when it was created and updated, and, if you subscribe,
            when your subscription runs until.
          </li>
          <li>
            <strong>What you write:</strong> posts, comments, reactions, and your conversations with the writing
            assistant.
          </li>
          <li>
            <strong>Payments.</strong> If you subscribe, Stripe collects your payment details on its own checkout
            page. The site keeps only your Stripe customer ID and when your paid access ends.
          </li>
          <li>
            <strong>Request logs.</strong> Every request to the backend is logged with your IP address and browser
            user agent.
          </li>
        </ul>

        <h2>Why, and on what legal basis</h2>
        <p>
          Your account, profile, content and subscription are processed to provide the service you signed up for
          (contract). Request logs, automated comment moderation, and search and related-post embeddings are
          processed to keep the site secure, free of abuse and useful to readers (legitimate interests). Nothing is
          used for advertising or sold.
        </p>

        <h2>Who processes it</h2>
        <ul>
          <li>
            <strong>Google Cloud</strong> hosts the backend (Cloud Run), stores accounts and content (Firestore),
            keeps request logs (Cloud Logging), and runs Gemini on Vertex AI. Drafts you share with the writing
            assistant are sent to Gemini, and so are published posts (to build search and related-post embeddings) and
            new comments (to moderate them).
          </li>
          <li>
            <strong>Google Identity Services</strong> handles signing in with Google.
          </li>
          <li>
            <strong>Cloudflare</strong> serves the website (Cloudflare Pages).
          </li>
          <li>
            <strong>Stripe</strong> takes subscription payments and runs the billing portal.
          </li>
        </ul>
        <p>
          These providers may process data outside the UK and EU, including in the United States, under their own
          safeguards for international transfers such as standard contractual clauses.
        </p>

        <h2>How long it is kept</h2>
        <p>
          Your profile and everything you created are kept until you delete them or your account. Request logs are
          kept for Cloud Logging's default retention of 30 days. Stripe keeps payment records for as long as the law
          requires it to.
        </p>

        <h2>Your rights</h2>
        <p>
          Under UK and EU data protection law you can ask for a copy of your data, have it corrected or erased,
          object to or restrict how it is processed, and take it elsewhere. You can edit your profile and content at
          any time. <strong>Delete account</strong> on your profile's edit page erases your profile, posts, comments,
          reactions and assistant conversations straight away. For anything else, get in touch as above. You can
          also complain to the Information Commissioner's Office (ico.org.uk) or your local data protection
          authority.
        </p>

        <h2>Cookies and browser storage</h2>
        <p>
          The site stores only what it needs to sign you in and the one preference you set. There are no analytics or
          advertising cookies, so there is nothing to consent to.
        </p>
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Set by</th>
              <th>Why</th>
              <th>How long</th>
            </tr>
          </thead>
          <tbody>
            {STORAGE_ITEMS.map((item) => (
              <tr key={item.name}>
                <td>
                  <code>{item.name}</code>
                </td>
                <td>{item.kind}</td>
                <td>{item.setBy}</td>
                <td>{item.purpose}</td>
                <td>{item.lasts}</td>
              </tr>
            ))}
          </tbody>
        </table>

        <h2>Changes</h2>
        <p>
          This policy changes when what the site does changes; the date at the top says when it last did. See also the{' '}
          <Link to="/terms">terms of service</Link>.
        </p>
      </div>
    </div>
  )
}
