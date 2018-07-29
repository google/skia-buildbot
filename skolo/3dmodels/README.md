3D Models
=========

This directory contains the the 3D model files in STL format for
3D printable designs to house a Raspberry Pi (RPi) and Android device
for testing.
The units of all models are in millimeters (mm). This is relevant when
importing them into a tool like Tinkercad.

The holder is designed to hold a RPi in this specific case:

  http://www.amazon.com/SB-Components-Clear-Case-Raspberry/dp/B00MQLB1N6

and (up through v5) this specific USB hub:

  http://www.amazon.com/Sabrent-Individual-Switches-included-HB-UMP3/dp/B00TPMEOYM

It would be very simple to adapt it to other cases and hubs.
The separate raspirack_pi_holder model is a good example for a stand alone
component that is specific to a case.

raspirack_v5 adds a way to connect two racks together. The extenders on the
sides have holes that line up and can be fastened with a
3/32 by 2.5 inch wire hair pin clip (aka cotter pin). Similar in shape to this:

  http://www.amazon.com/Koch-Industries-4022333-8-Inch-10-Piece/dp/B00OV7TO0U

Starting in v7, only 1 cotter pin on each side is required to join the racks together,
further narrowing the design.

Contents:
=========

raspirack_*.stl : Contains different versions of a rack to hold
                  a RaspberryPi (in a translucent case) with a dedicated
                  USB hub and an arbitrary Android devices.
                  Should work with any device up to a Nexus 10.

raspirack_*_extended.stl : A variant of the rack that has a larger "payload"
                           to accommodate bigger devices (e.g. NVIDIA Shield).
                           The payload width is 25mm, compared to 11mm on the
                           standard rack.

raspirack_*_narrow.stl : A variant of the rack with the same size "payload"
                          as normal, but has only one pin spot on each side
                          to connect with other racks.  Additionally, the
                          depth of the rack is about half of normal. This
                          should allow for more rpi density and faster batch
                          printing.


raspirack_pi_holder.stl : Holder for the RPi. This is incorporated into
                          all versions starting with raspirack_v4.
