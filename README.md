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

# Network Security
* **Traffic is always initiated by the agent to Neptune. No sessions are ever initiated from Neptune back to the agent**
* All traffic is sent over SSL
* The destination for all agent data is: www.neptune.io
It is a **CNAME**; its IP address is subject to change but belongs to the ranges:

``` 
23.20.0.0/14 (23.20.0.1 - 23.23.255.254)
50.16.0.0/15 (50.16.0.1 - 50.17.255.254)
50.19.0.0/16 (50.19.0.1 - 50.19.255.254)
52.0.0.0/15 (52.0.0.1 - 52.1.255.254)
52.2.0.0/15 (52.2.0.1 - 52.3.255.254)
52.20.0.0/14 (52.20.0.1 - 52.23.255.254)
52.4.0.0/14 (52.4.0.1 - 52.7.255.254)
52.70.0.0/15 (52.70.0.1 - 52.71.255.254)
52.72.0.0/15 (52.72.0.1 - 52.73.255.254)
52.86.0.0/15 (52.86.0.1 - 52.87.255.254)
52.90.0.0/15 (52.90.0.1 - 52.91.255.254)
52.95.245.0/24 (52.95.245.1 - 52.95.245.254)
52.95.255.80/28 (52.95.255.81 - 52.95.255.94)
54.144.0.0/14 (54.144.0.1 - 54.147.255.254)
54.152.0.0/16 (54.152.0.1 - 54.152.255.254)
54.156.0.0/14 (54.156.0.1 - 54.159.255.254)
54.160.0.0/13 (54.160.0.1 - 54.167.255.254)
54.172.0.0/15 (54.172.0.1 - 54.173.255.254)
54.174.0.0/15 (54.174.0.1 - 54.175.255.254)
54.196.0.0/15 (54.196.0.1 - 54.197.255.254)
54.198.0.0/16 (54.198.0.1 - 54.198.255.254)
54.204.0.0/15 (54.204.0.1 - 54.205.255.254)
54.208.0.0/15 (54.208.0.1 - 54.209.255.254)
54.210.0.0/15 (54.210.0.1 - 54.211.255.254)
54.221.0.0/16 (54.221.0.1 - 54.221.255.254)
54.224.0.0/15 (54.224.0.1 - 54.225.255.254)
54.226.0.0/15 (54.226.0.1 - 54.227.255.254)
54.234.0.0/15 (54.234.0.1 - 54.235.255.254)
54.236.0.0/15 (54.236.0.1 - 54.237.255.254)
54.242.0.0/15 (54.242.0.1 - 54.243.255.254)
54.80.0.0/13 (54.80.0.1 - 54.87.255.254)
54.88.0.0/14 (54.88.0.1 - 54.91.255.254)
54.92.128.0/17 (54.92.128.1 - 54.92.255.254)
67.202.0.0/18 (67.202.0.1 - 67.202.63.254)
72.44.32.0/19 (72.44.32.1 - 72.44.63.254)
75.101.128.0/17 (75.101.128.1 - 75.101.255.254)
107.20.0.0/14 (107.20.0.1 - 107.23.255.254)
174.129.0.0/16 (174.129.0.1 - 174.129.255.254)
184.72.128.0/17 (184.72.128.1 - 184.72.255.254)
184.72.64.0/18 (184.72.64.1 - 184.72.127.254)
184.73.0.0/16 (184.73.0.1 - 184.73.255.254)
204.236.192.0/18 (204.236.192.1 - 204.236.255.254)
216.182.224.0/20 (216.182.224.1 - 216.182.239.254)
```

To regenerate this list, you can run this small shell script [located here](https://gist.github.com/darron/811cf41a6ec3dbfcb97a).

[You can also subscribe to AWS Public IP Address Changes via Amazon SNS.](https://aws.amazon.com/blogs/aws/subscribe-to-aws-public-ip-address-changes-via-amazon-sns/)

# Contributing
* Fork it
* Create your feature branch (git checkout -b my-new-feature)
* Commit your changes (git commit -am 'Add some feature')
* Push to the branch (git push origin my-new-feature)
* Create new Pull Request
