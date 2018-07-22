groupadd --system gobgpd
useradd --system -d /var/lib/gobgpd -s /bin/bash -g gobgpd gobgpd
mkdir -p /var/{lib,run,log}/gobgpd
chown -R gobgpd:gobgpd /var/{lib,run,log}/gobgpd
mkdir -p /etc/gobgpd
chown -R gobgpd:gobgpd /etc/gobgpd
