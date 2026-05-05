// Static SMS terms-of-service. Like the privacy policy, the language
// here is shaped by US carrier requirements (CTIA Short Code Monitoring
// Handbook + A2P 10DLC) and by what Twilio's reviewers need to see in
// order to approve a 10DLC campaign.
export function TermsPage() {
  return (
    <article className="legal">
      <h1>SMS Terms of Service</h1>
      <p className="muted">Last updated: 2026-05-04</p>

      <section>
        <h2>Program description</h2>
        <p>
          GamePulse is an opt-in service that sends informational SMS
          messages about NBA, NFL, MLB, and NHL teams. After you sign up
          on the website you may receive:
        </p>
        <ul>
          <li>
            <strong>Live score updates</strong> — sent in realtime when
            the score of a game involving your selected team changes.
          </li>
          <li>
            <strong>News summaries</strong> — AI-summarised news headlines
            about your team, sent at the cadence you chose (daily, weekly,
            or monthly).
          </li>
          <li>
            <strong>Confirmation messages</strong> — a single welcome SMS
            sent immediately after you subscribe.
          </li>
        </ul>
      </section>

      <section>
        <h2>Message frequency</h2>
        <p>
          Frequency varies by team activity. During an active game you may
          receive several score updates within a few hours. News summaries
          are sent according to the cadence you chose at signup.
        </p>
      </section>

      <section>
        <h2>Costs</h2>
        <p>
          Message and data rates may apply. GamePulse does not charge for
          messages, but your mobile carrier's standard rates for SMS and
          data will apply.
        </p>
      </section>

      <section>
        <h2>Opt-in</h2>
        <p>
          You opt in by entering your phone number on the website,
          selecting a team, and explicitly agreeing to receive SMS
          messages by checking the consent checkbox. Submitting the form
          constitutes your agreement to receive messages from GamePulse.
        </p>
      </section>

      <section>
        <h2>Opt-out</h2>
        <p>
          Reply <code>STOP</code> to any GamePulse message at any time to
          cancel. After replying STOP you will receive a single
          confirmation message and no further messages will be sent. You
          can also remove a specific subscription via the website.
        </p>
      </section>

      <section>
        <h2>Help</h2>
        <p>
          Reply <code>HELP</code> to any GamePulse message for help, or
          email{' '}
          <a href="mailto:support@gamepulse.example">
            support@gamepulse.example
          </a>
          . Carrier-related issues should be directed to your wireless
          carrier.
        </p>
      </section>

      <section>
        <h2>Supported carriers</h2>
        <p>
          GamePulse delivers messages via Twilio. The service is supported
          on AT&amp;T, T-Mobile, Verizon, Sprint, U.S. Cellular, and other
          major U.S. wireless carriers. Wireless carriers are not liable
          for delayed or undelivered messages.
        </p>
      </section>

      <section>
        <h2>Privacy</h2>
        <p>
          See our{' '}
          <a href="#privacy">privacy policy</a> for how we handle your
          phone number and other information.
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
