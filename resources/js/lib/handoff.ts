/**
 * What the panel returns when the user opens a tool on a node (Adminer, webmail): the address of
 * the tool, the name of the form field it reads, and a one-time token. The token is sent in the
 * body of a POST, never put in the address, so it is not left in the node's access log, the
 * browser history or a Referer header.
 */
export type Handoff = {
    url: string;
    field: string;
    token: string;
};

/**
 * Submits the hand-off as a form POST, into `tab` when one was opened (opened before the request so
 * the browser's popup blocker allows it) and in this window otherwise.
 */
export function submitHandoff(handoff: Handoff, tab: Window | null): void {
    const form = document.createElement('form');
    form.method = 'post';
    form.action = handoff.url;
    form.style.display = 'none';

    if (tab) {
        // A window opened with window.open('', '_blank') has no name to target; give it one, then
        // cut its link back to this page.
        tab.name = `lesta-handoff-${crypto.randomUUID()}`;
        tab.opener = null;
        form.target = tab.name;
    }

    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = handoff.field;
    input.value = handoff.token;
    form.appendChild(input);

    document.body.appendChild(form);
    form.submit();
    form.remove();
}
