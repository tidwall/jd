<p align="center">
<img 
    src="https://github.com/tidwall/jd/wiki/images/logo.png" 
    width="101" height="78" border="0" alt="SJSON">
<br>
JSON Interactive Editor
</p>

JD is an experimental tool for querying and editing JSON documents.
It's basically a playground to show off the [GJSON](https://github.com/tidwall/gjson) path syntax. 

![demo-basic](https://github.com/tidwall/jd/wiki/images/demo-basic.gif)

It's possible to add, delete, and edit any JSON value type.

![demo-elements](https://github.com/tidwall/jd/wiki/images/demo-elements.gif)


## Usage

```bash
# Read from Stdin
echo '{"id":9851,"name":{"first":"Tom","last":"Anderson"},"friends":["Sandy","Duke","Sam"]}' | jd

# Read from cURL
curl -s https://api.github.com/repos/tidwall/tile38/issues/23 | jd

# Read from a file
jd user.json
```

## Install

There're pre-built binaries for Mac, Linux, FreeBSD and Windows on the releases page.

### Mac (Homebrew)

```
brew tap tidwall/jd
brew install jd
```

### Build

```
go get -u github.com/simeji/jid/cmd/jid
```


## Contact
Josh Baker [@tidwall](http://twitter.com/tidwall)

## License

JS source code is available under the MIT [License](/LICENSE).
