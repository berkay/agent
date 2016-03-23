# Neptune agent
Neptune agent is a lightweight piece of software written in GO, which runs on your servers to run runbooks and report the results back to Neptune. We use an agent to accomplish this because it provides an easy way to run runbooks without exposing any open ports and without requiring SSH access to your servers.

# Installation
Documentation is available at http://docs.neptune.io/docs/agent-installation

# Administration
Please see http://docs.neptune.io/docs/agent-administration

# Architecture white paper 
This document has more details about threat model and different attack vectors. Please see https://s3.amazonaws.com/prod-neptuneio-downloads/security/agent/NeptuneArchitectureWhitePaper.pdf

# Agent FAQs
Please see FAQs at http://docs.neptune.io/docs/agent-faqs

# Architecture
![Agent Architecture](/NeptuneAgentArchitecture.png?raw=true)

Read more at http://docs.neptune.io/docs/agent-faqs#architecture-and-more

# Contributing
* Fork it
* Create your feature branch (git checkout -b my-new-feature)
* Commit your changes (git commit -am 'Add some feature')
* Push to the branch (git push origin my-new-feature)
* Create new Pull Request
