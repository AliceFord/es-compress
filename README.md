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
magic value   4 bytes ("esco")
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
[timestamp]
[generic record]
...
[generic record]
[stop]
... ([timestamp] -> [stop] repeated)
[stop]
```

#### `[timestamp]`

```
time in s  3 bytes (uint)
```

#### `[generic record]`

```
record type  1 byte
[record of type [0]]
```

#### `[record] (type 0): position record`

```
transponder type   1 byte
position id        1 byte
aircraft id        2 bytes
squawk             2 bytes  (uint)
lat                4 bytes
lon                4 bytes
alt                2 bytes (uint)
hdg                2 bytes (uint)
```

#### `[record] (type 1): position record (delta only)`

```
change map                   1 byte
position id                  1 byte
aircraft id                  2 bytes
(maybe) new transponder type 1 byte
(maybe) new squawk           2 bytes (uint)
(maybe) lat delta            2 bytes
(maybe) lon delta            2 bytes
(maybe) alt delta            2 bytes
(maybe) hdg delta            1 byte
```
