# Euroscope replay file encoder + decoder

Lossy(*) storage for ES replay files. 

Supported record types:
```
@N/@S (position)
#TM (text message)
#AP (add pilot)
(all below are supported uncompressed)
```
Planned record types for compression:
```
% (ATC position message)
#AA (add ATC)
$FP (flightplan)
#PC (coordination)
#DP (delete pilot)
$CR (request details)
$AR (METAR response)
$ER (errors)
$HO / $HA (handoff offer + accept)
@Y (squawk ident)
```
Ignored record types (as these are essentially useless):
```
$CQ (client query, which are just protocol requests and hold no useful data)
$SB (ac model matching from SquawkBox)
$ZC / $ZR (client authentication)
#ST (fast packet, not used by ATC client)
$AX (ask for METAR)
#TM FP GET (flightplan recieved ack)
```

*: Lossy because these "useless" packets are discarded. All supported record types are stored lossless.

## Current file format (vibes-based diagram)

#### Overall file structure

```
[header][text registry][record stream]
```

#### `[header]`

```
magic value   4 bytes ("skog")
version       1 byte
```

#### `[text registry]`

```
[text]
...
[text]
[stop]
```

#### `[text]`

```
string followed by 0 byte
```

#### `[stop]`

```
0 byte
```

#### `[record stream]`

```
[generic record]
...
[generic record]
[FF]
```

#### `[generic record]`

```
record type  1 byte
[record of type [0]]
```

#### `[record] (type 0): position record`

```
transponder type  1 byte  (bool)
aircraft id       ? bytes (uvarint)
squawk            2 bytes (uint)
lat               4 bytes (int, actual lat * 100000)
lon               4 bytes (int, actual lon * 100000)
alt               2 bytes (uint)
hdg               2 bytes (uint)
```

#### `[record] (type 1): position record (delta only)`

```
[change map]                   1 byte 
aircraft id                  ? bytes (uvarint)
(maybe) new transponder type 1 byte  (bool)
(maybe) new squawk           2 bytes (uint)
(maybe) lat delta            ? bytes (varint)
(maybe) lon delta            ? bytes (varint)
(maybe) alt delta            ? bytes (varint)
(maybe) hdg delta            ? bytes (varint)
```

##### `[change map]`

```
low bit = bit 0
bit 0: (maybe) new transponder type
bit 1: (maybe) new squawk
bit 2: (maybe) lat delta 
bit 3: (maybe) lon delta 
bit 4: (maybe) alt delta
bit 5: (maybe) hdg delta
bit 6: reserved
bit 7: reserved
```

#### `[record] (type 2): controller position change`

```
position id  1 byte
```

#### `[record] (type 3): timestamp`

```
time in s  3 bytes (uint)
```

#### `[record] (type 4): timestamp +1s`

(Note this record has no content.)

#### `[record] (type 5): type 1, with change map none`

#### `[record] (type 6): type 1, with change map lat lon`

#### `[record] (type 7): type 1, with change map lat lon alt`

#### `[record] (type 8): type 1, with change map lat lon alt hdg`

#### `[record] (type 9): type 1, with change map lat lon hdg`

#### `[record] (type 10): message`

```
sender id    ? bytes (uvarint)
receiver id  ? bytes (uvarint)
message      ? bytes (cstring)
```

#### `[record] (type 11): unknown`

```
[arrow type]
unknown packet details  ? bytes (cstring)
```

##### `[arrow type]`
```
1 byte, either:
00    === ">>>>"
01    === "<<<2"
02    === "2>>1"
```

#### `[record] (type 12): add pilot packet`

```
pilot id     ? bytes (uvarint)
cid          3 bytes (uint)
rating       1 byte  (uint)
name         ? bytes (cstring)
```