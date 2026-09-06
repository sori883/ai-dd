# Intent-capture question-rendering annex

Render one question per turn. A question is paired with one fresh HUMAN_TURN
and then one exact answer; never reuse a turn for another decision. Keep the
authoritative project description at `project-description.json` and use the
delivered context reader for every other path. There is no path exploration,
directory listing, or caller-selected replacement path.

The consolidated summary is a separate question. Its accepted answer is the
exact text `Looks correct`; any other answer keeps the summary unconfirmed.
Approval likewise accepts exact `Approve` or `Request Changes` only. After a
sensor run, render the mandatory learnings question on a new fresh HUMAN_TURN.
