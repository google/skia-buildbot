Role Name
=========

Disables Gatekeeper on Macs.

Gatekeeper is an application that disallows running any executable downloaded
from the web. We sometimes need to disable it, which this role does.

Requirements
------------

N/A

Role Variables
--------------

N/A

Dependencies
------------

N/A

Example Playbook
----------------

    - hosts: compui
      roles:
         - disable-gatekeeper

