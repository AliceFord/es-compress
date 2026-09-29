# Euroscope replay file encoder + decoder

Lossy storage for ES replay files. 

v0 only includes aircraft position updates, i.e. `@N` / `@S`. This is because these are all that are needed for making replays. 

Planned (though probably will never get round to it), is to expand to include more messages, including to make a lossless format.

## Current file format (vibes-based diagram)

#### Overall file structure

```
[header][callsign registry][aircraft registry][record stream]
```

#### `[header]`

```
magic value   4 bytes ("skog")
version       1 byte
```

#### `[callsign registry]`

```
[callsign]
...
[callsign]
[stop]
```

#### `[callsign]`

```
[A-Z0-9_]+ followed by 0 byte
```

#### `[stop]`

```
0 byte
```

#### `[aircraft registry]`

```
[callsign]
...
[callsign]
[stop]
```

#### `[callsign]`

```
[A-Z0-9]+ followed by 0 byte
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
aircraft id       2 bytes (uint)
squawk            2 bytes (uint)
lat               4 bytes (int, actual lat * 100000)
lon               4 bytes (int, actual lon * 100000)
alt               2 bytes (uint)
hdg               2 bytes (uint)
```

#### `[record] (type 1): position record (delta only)`

```
[change map]                   1 byte 
aircraft id                  2 bytes (uint)
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
