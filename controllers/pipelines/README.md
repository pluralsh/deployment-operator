# Assumptions
- gate at console can only become open or closed when the reconciler reports it as such, or in other words, it cannot be closed or opened manually
# logic
initial reconciliation: set state to to syncedstate	

syncedstate: 
    - pending: 
        - if lastreported is pending: create job to update state
        - if lastreported is open: create job to update state
        - if lastreported is closed: create job to update state
    - open:
        - if lastreported is pending: create job to update state
        - if lastreported is open: create job to update state
        - if lastreported is closed: create job to update state
    - closed:
        - if lastreported is pending: create job to update state
        - if lastreported is open: create job to update state
        - if lastreported is closed: create job to update state

pending means pending:
make sure the state is always synced with the console, i.e. the lastreported state is the same as the state at the console:
- if the state is pending, then reconcile the job to reach either open or closed and report
- also update the the console state to pending
- ignore an "abort" e.g. the console state is open/closed but the CR state is pending

why no abort?:
e.g. current state is pending but new syncedstate is open or close
problem:
- console state is only synced, i.e. PULLED from the console, every minute or so, so in fairly long intervals
- in general any state change from the cluster will make it faster to the console than the other way around, because it PUSHES the info to the console
- if you want to allow a user to manually change the state of the gate in the console, e.g. via UI from PENDING to CLOSED (because you want to abort), and the controller already ran the reconcile loop, then the controller will overwrite the state of the gate at the console from CLOSED to PENDING again, because it is the last state it knows about
- what you really need is a DISABLED state, that blocks the gate at the console entirely from updates by the controller

rerun?:
e.g. current state of CR is open or closed but new syncedstate is pending
- check if the last reported state is the same as the current state, so it should have been reported to have closed or open
- check if the last reported timestamp is older than the current synced timestamp, so it should have been reported to have closed or open 




case: synced after lastreported
    synced pending -> state pending -> handle job to update state
    synced pending -> state open -> handle job to update state
    synced pending -> state open -> handle job to update state
    pending -> closed
case: synced after lastreported
    state -> open
    state -> closed

table with 4 columns and 10 rows

| spec.synced | status.state | status.lastreported | action |
|-------------|--------------|---------------------|--------|
| pending     | pending      | pending             | create and/or wait for job        |
| pending     | pending      | open                |        |
| pending     | pending      | closed              |        |
| pending     | open         | pending             |        |
| pending     | open         | open                |        |
| pending     | open         | closed              |        |
| pending     | closed       | pending             |        |
| pending     | closed       | open                |        |
| pending     | closed       | closed              |        |
| open        | pending      | pending             |        |
| open        | pending      | open                |        |
| open        | pending      | closed              |        |
| open        | open         | pending             |        |
| open        | open         | open                |        |
| open        | open         | closed              |        |
| open        | closed       | pending             |        |
| open        | closed       | open                |        |
| open        | closed       | closed              |        |
| closed      | pending      | pending             |        |
| closed      | pending      | open                |        |
| closed      | pending      | closed              |        |
| closed      | open         | pending             |        |
| closed      | open         | open                |        |
| closed      | open         | closed              |        |
| closed      | closed       | pending             |        |
| closed      | closed       | open                |        |
| closed      | closed       | closed              |        |
