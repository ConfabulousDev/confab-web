This Cookie Policy explains how Confabulous Software LLC ("**Company**," "**we**," "**us**," and "**our**") uses cookies and similar technologies when you visit [https://confabulous.dev](https://confabulous.dev) (the "**Website**") and our documentation site at [https://docs.confabulous.dev](https://docs.confabulous.dev) (the "**Docs**"). It explains what these technologies are, which ones we use and why, and how you can control them.

## What are cookies and similar technologies?

Cookies are small data files that a website places on your computer or mobile device when you visit it. Websites can also keep small amounts of data in your browser using browser storage (`localStorage`, which persists until it is cleared, and `sessionStorage`, which is cleared when you close the browser tab).

Cookies and storage set by the website you are visiting (in this case, Confabulous Software LLC) are called "first-party." Those set by other parties are called "third-party."

## What do we use?

We use only first-party cookies and browser storage that are strictly necessary to provide the Website and the Docs: to keep you signed in, to protect the sign-in process, and to remember a small number of display preferences you choose. We do not use advertising cookies, we do not use cookies to track you across other websites, and we do not set third-party cookies. Because we only use strictly necessary cookies and storage, we do not ask for cookie consent.

All of our cookies are sent only to confabulous.dev, are marked `HttpOnly` (not readable by scripts on the page), `Secure` (sent only over HTTPS), and `SameSite=Lax`.

### Cookies on the Website

| Name | Type | Purpose | Lifetime |
| --- | --- | --- | --- |
| `confab_session` | Cookie | Keeps you signed in. It holds a random identifier; your session details are stored on our servers. | 7 days. The sign-in it identifies ends sooner if you sign out or are inactive for a period of time. |
| `oauth_state` | Cookie | Protects sign-in with an external provider (such as GitHub or Google) against cross-site request forgery. | 5 minutes |
| `oauth_verifier` | Cookie | Secures the sign-in exchange with an external provider. | 5 minutes |
| `post_login_redirect` | Cookie | Returns you to the page you were on after you sign in. Set only when sign-in starts from a specific page. | 5 minutes |
| `expected_email` | Cookie | Checks that you sign in with the email address an invitation or share link was sent to. Set only when you follow such a link. | 5 minutes |
| `cli_redirect` | Cookie | Completes sign-in for the Confabulous command-line tool. | 5 minutes |

On our public demo at [https://demo.confabulous.dev](https://demo.confabulous.dev), visitors share one read-only demo account, and `confab_session` identifies that shared demo session. There it lasts up to 100 years.

### Browser storage on the Website

| Name | Type | Purpose | Lifetime |
| --- | --- | --- | --- |
| `confab-theme` | localStorage | Remembers whether you chose the light or dark theme. | Until you clear it |
| `quickstart-cta-dismissed` | localStorage | Remembers that you dismissed the quickstart prompt. | Until you clear it |
| `chunk_reload_attempted:<page>` | sessionStorage | Prevents a reload loop if part of the page fails to load after we release an update. | Until you close the tab; removed once the page loads |

### Browser storage on the Docs

The Docs set no cookies.

| Name | Type | Purpose | Lifetime |
| --- | --- | --- | --- |
| `starlight-theme` | localStorage | Remembers whether you chose the light or dark theme. | Until you clear it |
| `sl-sidebar-state` | sessionStorage | Remembers the state of the navigation sidebar as you move between pages. | Until you close the tab |

## Do you use analytics?

The Website does not use analytics cookies or analytics services.

The Docs use [Cloudflare Web Analytics](https://www.cloudflare.com/web-analytics/) to count page views in aggregate. Your browser loads a small script from Cloudflare when you visit the Docs. Cloudflare states that Web Analytics does not use cookies or browser storage to collect usage data and does not fingerprint individual visitors. See [Cloudflare's privacy policy](https://www.cloudflare.com/privacypolicy/) for details.

## Do you use third-party fonts?

No. The fonts on the Website and the Docs are served from the same site as the page, so loading a page does not send a request to a third-party font provider.

## How can I control cookies on my browser?

Most browsers let you view, block and delete cookies and site data. If you block or delete the cookies above, you will be signed out and may not be able to sign in until you allow them again. As the means by which you can refuse cookies through your web browser controls vary from browser to browser, you should visit your browser's help menu for more information. The following is information about how to manage cookies on the most popular browsers:

- [Chrome](https://support.google.com/chrome/answer/95647#zippy=%2Callow-or-block-cookies)
- [Firefox](https://support.mozilla.org/en-US/kb/enhanced-tracking-protection-firefox-desktop?redirectslug=enable-and-disable-cookies-website-preferences&redirectlocale=en-US)
- [Safari](https://support.apple.com/en-ie/guide/safari/sfri11471/mac)
- [Edge](https://support.microsoft.com/en-us/windows/microsoft-edge-browsing-data-and-privacy-bb8174ba-9d73-dcf2-9b4a-c582b4e640dd)
- [Opera](https://help.opera.com/en/latest/web-preferences/)

## Self-hosted instances

Confabulous can also be self-hosted. This Cookie Policy covers only the Website, the Docs and the demo, which we operate. A self-hosted instance is operated by whoever runs it, not by us.

## How often will you update this Cookie Policy?

We may update this Cookie Policy from time to time, for example to reflect changes to the cookies and storage we use, or for other operational, legal, or regulatory reasons. The date at the top of this Cookie Policy indicates when it was last updated.

## Where can I get further information?

If you have any questions about our use of cookies or other technologies, please email us at [contact@confabulous.dev](mailto:contact@confabulous.dev) or write to us at:

Confabulous Software LLC

1315 Piedmont Rd, #32281

San Jose, CA 95152

United States
