// Static privacy policy. Content is intentionally explicit about SMS
// data handling because Twilio's A2P 10DLC reviewers (and US carriers)
// look for these specific disclosures before approving a campaign.
export function PrivacyPage() {
  return (
    <article className="legal">
      <h1>Privacy Policy</h1>
      <p className="muted">Last updated: 2026-05-04</p>

      <section>
        <h2>Information we collect</h2>
        <p>
          GamePulse collects only the information necessary to deliver the
          service you signed up for: your phone number (used to send SMS
          updates), the team you selected, the type of updates you chose
          (live, news, or both), and your chosen news-summary cadence
          (daily, weekly, or monthly).
        </p>
      </section>

      <section>
        <h2>How we use your phone number</h2>
        <p>
          Your phone number is used <strong>only</strong> to deliver the
          score and news SMS messages you opted into. We do <strong>not</strong>
          {' '}sell, rent, lease, or otherwise share your phone number or
          other personal information with third parties for marketing
          purposes. Phone numbers are not shared with affiliates for
          marketing or promotional purposes.
        </p>
      </section>

      <section>
        <h2>Service providers</h2>
        <p>
          We use the following service providers strictly to operate the
          service:
        </p>
        <ul>
          <li>
            <strong>Twilio</strong> — to deliver SMS messages to your
            carrier.
          </li>
          <li>
            <strong>ESPN</strong> public scoreboard — to fetch live game
            data. No personal data is shared.
          </li>
          <li>
            <strong>NewsAPI</strong> — to fetch sports news headlines. No
            personal data is shared.
          </li>
          <li>
            <strong>OpenAI</strong> — to summarise news articles for SMS
            delivery. No personal data is shared.
          </li>
        </ul>
      </section>

      <section>
        <h2>Your choices</h2>
        <p>
          You can stop receiving messages at any time by replying{' '}
          <code>STOP</code> to any message we send. We will immediately
          remove all of your subscriptions and stop sending you messages.
          You can also delete a specific subscription via the website.
        </p>
      </section>

      <section>
        <h2>Data retention</h2>
        <p>
          We retain your phone number and subscription preferences for as
          long as your subscription is active. After you unsubscribe (via
          STOP or via the website), we retain a record of the unsubscribe
          event to honor opt-out preferences but will not send you further
          messages.
        </p>
      </section>

      <section>
        <h2>Contact</h2>
        <p>
          Questions? Email{' '}
          <a href="mailto:support@gamepulse.example">
            support@gamepulse.example
          </a>
          .
        </p>
      </section>

      <p>
        <a href="#home" className="btn btn--ghost">
          ← Back
        </a>
      </p>
    </article>
  );
}
