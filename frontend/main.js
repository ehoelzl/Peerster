const {h, Component, render} = preact;
const html = htm.bind(h);
const uiAddress = "http://127.0.0.1:8080"


function hexStringToByte(str) {
    if (!str) {
        return new Uint8Array();
    }

    var a = [];
    for (var i = 0, len = str.length; i < len; i+=2) {
        a.push(parseInt(str.substr(i,2),16));
    }

    return new Uint8Array(a);
}

class Node extends Component {
    state = {nodeString: ""};

    refreshNode() {
        $.getJSON(uiAddress + "/id", function (data) {
            this.setState(({gossipAddress, name}) => (data))
            this.setState(({disconnected}) => ({disconnected: false}))
        }.bind(this)).fail(function () {
            this.setState({disconnected: true, name: "", gossipAddress: ""})
        }.bind(this))
    }

    componentDidMount() {
        this.refreshNode()
        setInterval(() => this.refreshNode(), 15 * 1000)
    }

    NodeString(props) {
        if (this.state.disconnected) {
            return html`<div>Node disconnected</div>`
        } else {
            return html`
              <div style="float: left; margin-right: 10px">Node</div>
              <div style="font-weight: bold" >${this.state.name}</div>
              <div style="float: left; margin-right: 10px">Gossip Address</div>
              <div class="peers" >${this.state.gossipAddress} </div>
      `
        }
    }


    render() {
        return html`
        <section>
            <div style="overflow: hidden">
              ${this.NodeString()}
            </div>
        </section>
    `
    }
}

class NodesBox extends Component {
    state = {nodes: [], newNode: ""};

    refreshNodes() {
        $.getJSON(uiAddress + '/node', function (data) {
            this.setState(({nodes}) => ({nodes: data}))
        }.bind(this)).fail(function () {
            this.setState(({nodes}) => ({nodes: []}))
        }.bind(this))
    }

    handleChange(event) {
        this.setState(({newNode}) => ({newNode: event.target.value}))
    }

    componentDidMount() {
        this.refreshNodes()
        setInterval(() => this.refreshNodes(), 15 * 1000)
    }

    handleSubmit(event) {

        $.ajax({
            type: 'POST',
            url: uiAddress + '/node',
            async: false,
            data: JSON.stringify({"Text": this.state.newNode}),
            contentType: "application/json",
            dataType: 'json'
        }).fail((d, ts, xhr) => {
            if (d.status == 400) {
                alert("Could not resolve address")
            }
        });
        this.setState(({newNode}) => ({newNode: ""}))
        this.refreshNodes()
        event.preventDefault()
    }

    render() {
        return html`
      <section>
        <div style="overflow: hidden">
             <div style="float: left;margin-right: 20px;font-weight: bold">Nodes</div>
             <div style="float: left" class="is-special"> ${this.state.nodes.length}</div>
        </div>
        <hr style="border-top: 2px"/>
        <div class="peers" style="margin-bottom: 20px; max-height: 75px; overflow-y: scroll">
            ${this.state.nodes.sort().map(node => {
            return (html`<div>${node}</div>`);
        })}
            
        </div>
        <hr/>
        <div>
            <div style="font-weight: bold; margin-bottom: 20px">Add a new node</div>
            <form style="width: 300px" onSubmit=${this.handleSubmit.bind(this)}>
                <label style="font-weight: normal">
                    Address:
                 <input type="text" value=${this.state.newNode} onChange=${this.handleChange.bind(this)} style="background: white"/>
                </label>
                <input type="submit" value="Add" class="button"/>
            </form>
        </div>
       
      </section>
    `
    }
}

class MessagesBox extends Component {
    state = {messages: {}, privateMessages: {}}

    refreshMessages() {
        $.getJSON(uiAddress + '/message', function (data) {
            this.setState(({messages}) => ({messages: data}))
        }.bind(this))
        $.getJSON(uiAddress + '/private', function (data) {
            this.setState(({privateMessages}) => ({privateMessages: data}))
        }.bind(this))
    }

    componentDidMount() {
        this.refreshMessages();
        setInterval(() => this.refreshMessages(), 1 * 1000)
    }

    printNodeMessages() {
        var items = [];
        for (var node in this.state.messages) {
            var messageIds = Object.keys(this.state.messages[node]).sort()
            var numMessages = messageIds.length
            for (let _id = 0; _id < numMessages; _id++) {
                var messageId = messageIds[_id]
                var message = this.state.messages[node][messageId]
                items.push(html`<div class="message">
                              <div style="width: 50%;font-family: monospace; float: left; height: 100%">${node}</div>
                              <div style="width: 50%; float: right">${message}</div>
                          </div>`)
            }
            items.push(html`<hr style="border-top: 1px dotted"/>`)
        }
        return items
    }

    printPrivateMessages() {
        var items = [];
        for (var node in this.state.privateMessages) {
            var messages = this.state.privateMessages[node]
            var numMessages = messages.length
            for (let _id = 0; _id < numMessages; _id++) {
                var message = messages[_id]
                items.push(html`<div class="message">
                              <div style="width: 50%;font-family: monospace; float: left; height: 100%">${node}</div>
                              <div style="width: 50%; float: right">${message}</div>
                          </div>`)
            }
            items.push(html`<hr style="border-top: 1px dotted"/>`)
        }
        return items
    }

    render() {
        return html`
        <section>
          <div style="padding-bottom: 20px">
            <div style="font-weight: bold; margin-bottom: 20px">Rumor Messages</div>
            <div style="margin-bottom: 15px; border-bottom: 2px dotted">
                <div style="width: 220px;float: left; font-family: monospace">From</div>
                <div>Message</div>
            </div>
            <div class="messages">
                ${this.printNodeMessages()}        
            </div>
          </div>
           <hr/>
           <div style="height: 50%">
            <div style="font-weight: bold; margin-bottom: 20px">Private Messages</div>
            <div style="margin-bottom: 15px; border-bottom: 2px dotted">
                <div style="width: 220px;float: left; font-family: monospace">From</div>
                <div>Message</div>
            </div>
            <div class="messages">
                ${this.printPrivateMessages()}        
            </div>
            
           </div>
        </section>
    `
    }
}

class ChatBox extends Component {
    state = {message: ""};

    handleChange(event) {
        this.setState(({message}) => ({message: event.target.value}))
    }

    handleSubmit(event) {
        event.preventDefault()
        if (this.state.message.length == 0) {
            alert("Cannot send empty message")
            return
        }
        $.ajax({
            type: 'POST',
            url: uiAddress + '/message',
            async: false,
            data: JSON.stringify({"Text": this.state.message}),
            contentType: "application/json",
            dataType: 'json'
        });
        this.setState(({message}) => ({message: ""}));
    }

    render() {
        return html`
        <div style="font-weight: bold">Rumor Chat Box</div>
        <hr style="border-top: 2px"/>
        <form style="width: 100px" onSubmit=${this.handleSubmit.bind(this)}>
            <textarea value=${this.state.message} onChange=${this.handleChange.bind(this)} style="background: white; resize: none; height:150px; width: 270%"/>
            <input type="submit" value="Send" class="button"/>
        </form>
    `
    }
}

class MessagePopUp extends Component {
    state = {message: ""}

    handleChange() {
        this.setState(({message}) => ({message: event.target.value}))
    }

    render() {
        return html`
        <div class='popup'>
          <div class='popup\_inner'>
            <div style="text-align: center; margin-top: 30px; margin-bottom: 15px">Send private message to ${this.props.destination}</div>
            <textarea value=${this.state.message} onChange=${this.handleChange.bind(this)} style="background: white; width: 50%; height: 100px; resize: none"/>
            <p>
            <button onClick=${() => this.props.send(this.props.destination, this.state.message)} style="width: 200px;">Send</button>
            <button onClick=${() => this.props.cancel()} style="width: 200px">Cancel</button>
            </p>
          </div>
        </div>
    `
    }
}

class FileRequestPopUp extends Component {
    state = {filename: "", filehash : ""}

    handleChange() {
        if (event.target.name  == "filename"){
            this.setState(({filename}) => ({filename : event.target.value}))
        } else {
            this.setState(({filehash}) => ({filehash : event.target.value}))
        }
    }

    render() {
        return html`
        <div class='popup'>
          <div class='popup\_inner'>
            <div style="text-align: center; margin-top: 30px; margin-bottom: 15px">Request file from ${this.props.destination}</div>
            <form>
                <label>
                    Filename
                    <input type="text" style="width: 40%; margin-left: 10px" value=${this.state.filename} onchange=${this.handleChange.bind(this)} name="filename" />
                </label>
                <label>
                    File Hash
                    <input type="text" style="width: 40%; margin-left: 10px" value=${this.state.filehash} onchange=${this.handleChange.bind(this)} name="filehash" />
                </label>
            </form>
            <p>
            <button onClick=${() => this.props.send(this.props.destination, this.state.filename, this.state.filehash)} style="width: 200px;">Request</button>
            <button onClick=${() => this.props.cancel()} style="width: 200px">Cancel</button>
            </p>
          </div>
        </div>
    `
    }
}

class Origins extends Component {
    state = {peers: [], messagePopUp: false, destination: null, fileRequestPopUp: false};

    refreshOrigins() {
        $.getJSON(uiAddress + '/origins', function (data) {
            if (data != null) {
                this.setState(({peers}) => ({peers: data.sort()}))
            }
        }.bind(this)).fail(function () {
            this.setState(({peers}) => ({peers: []}))
        }.bind(this))
    }

    componentDidMount() {
        this.refreshOrigins()
        setInterval(() => this.refreshOrigins(), 1 * 1000)
    }

    shouldComponentUpdate(nextProps, nextState) {
        return !(this.state.messagePopUp && nextState.messagePopUp) && !(this.state.fileRequestPopUp && nextState.fileRequestPopUp)
    }

    sendPrivateMessage(to, message) {
        if (message.length == 0) {
            alert("Cannot send empty private message")
        } else {
            $.ajax({
                type: 'POST',
                url: uiAddress + '/message',
                async: false,
                data: JSON.stringify({"Text": message, "Destination": to}),
                contentType: "application/json",
                dataType: 'json',
                error: (d, t, x) => {
                    if (d.status == 400) {
                        alert("Could not send private message")
                    } else if (d.status == 200) {
                        alert("Success")
                    }
                }
            })
        }
        this.toggleMessagePopUp()
    }

    requestFile(to, filename, filehash) {
        if (filename.length == 0) {
            alert("Please specify a filename")
        } else if (filehash.length == 0) {
            alert("Please specify a hash")
        } else {
            $.ajax({
                type: 'POST',
                url: uiAddress + '/message',
                async: false,
                data: JSON.stringify({"File": filename, "Destination": to, "Request": Array.from(hexStringToByte(filehash))}),
                contentType: "application/json",
                dataType: 'json',
                error: (d, t, x) => {
                    if (d.status == 400) {
                        alert("Could not send private message")
                    } else if (d.status == 200) {
                        alert("Success")
                    }
                }
            })
        }
        this.toggleFileRequestPopUp()
    }

    renderPeerButton(peer) {
        return html`
          <div class="message">
            <div style="width: 40%;font-family: monospace; float: left; height: 100%">${peer}</div>
            
            <div style="width: 60%; float: right; overflow: hidden">
            <button style="font-size: 10px; float: left; border-radius: 3px;" onClick=${
            () => {
                    this.setState(({destination}) => ({destination: peer}));
                    this.toggleMessagePopUp()
                }
            } >Send Private Message</button>
            
            <button style="overflow: right; font-size: 10px; border-radius: 3px;" onClick=${
            () => {
                    this.setState(({destination}) => ({destination: peer}));
                    this.toggleFileRequestPopUp()
                }
            }>Request File</button>
            </div>
            </div>
    `
    }

    toggleMessagePopUp() {
        this.setState(({messagePopUp}) => ({messagePopUp: !this.state.messagePopUp}))
    }

    toggleFileRequestPopUp() {
        this.setState(({fileRequestPopUp}) => ({fileRequestPopUp: !this.state.fileRequestPopUp}))
    }

    renderPopUp() {
        if (this.state.messagePopUp) {
            return html`<${MessagePopUp} destination=${this.state.destination} send=${this.sendPrivateMessage.bind(this)} cancel=${this.toggleMessagePopUp.bind(this)}/>`
        } else if (this.state.fileRequestPopUp) {
            return html`<${FileRequestPopUp} destination=${this.state.destination} send=${this.requestFile.bind(this)} cancel=${this.toggleFileRequestPopUp.bind(this)}/>`
        }
    }

    render() {
        return html`
      <section>
        ${this.renderPopUp()}
        <div style="overflow: hidden">
               <div style="float: left;margin-right: 20px;font-weight: bold">Known Peers</div>
               <div style="float: left" class="is-special"> ${this.state.peers.length}</div>
        </div>
        <hr style="border-top: 2px"/>
        <div class="peers" style="margin-bottom: 20px; overflow-y: scroll; max-height: 100px">
            ${this.state.peers.map(p => this.renderPeerButton(p))}
        </div>
      </section>
    `
    }
}

class ShareFile extends Component {

    handleChange(e) {
        e.preventDefault()
        $.ajax({
            type: 'POST',
            url: uiAddress + '/message',
            async: false,
            data: JSON.stringify({"File": e.target.files[0].name}),
            contentType: "application/json",
            dataType: 'json',
            error: (d, t, x) => {
                if (d.status == 400) {
                    alert("Could not index file")
                } else if (d.status == 200) {
                    alert("File indexed successfully")
                }
            }
        })
        e.target.value = null
    }

    render() {
        return html`
      <section>
        <div style="overflow: hidden">
               <div style="float: left;margin-right: 20px;font-weight: bold">Share File</div>
        </div>
        <hr style="border-top: 2px"/>
        <input type="file" name="file" onChange=${this.handleChange.bind(this)}/>
      </section>
    `
    }
}

class Files extends Component {
    state = {files : {}}

    refreshFiles() {
        $.getJSON(uiAddress + '/files', function (data) {
            this.setState(({files}) => ({files: data}))
        }.bind(this))
    }

    componentDidMount() {
        this.refreshFiles();
        setInterval(() => this.refreshFiles(), 5 * 1000)
    }

    renderFiles() {
        var items = [];
        for (var fileHash in this.state.files) {
            var file = this.state.files[fileHash]
            var filename = file.Filename
            var size = file.Size
            items.push(html`
                <div class="file">               
                    <div style="margin-bottom: 5px">${filename} (${size} Bytes)</div>
                    <div style="font-size: 12px">${file.Directory}, ${fileHash}</div>
                </div>
            `)

            return items
        }
    }

    render() {
        return html`
        <section>
          <div style="padding-bottom: 20px">
            <div style="font-weight: bold; margin-bottom: 20px">Indexed files</div>
             <div>
                ${this.renderFiles()}        
            </div>
          </div>
        </section>
    `
    }
}
class App extends Component {
    render() {
        return html`
      <header class="container">
        <h1 style="padding-left: 50px">Peerster Application</h1>
      </header>
      
      <section>
        <div class="wrapper">
          <div class="box a"><${Node}/></div>
          <div class="box b"><${MessagesBox}/></div>
          <div class="box c"><${NodesBox}/></div>
          <div class="box d"><${ChatBox}/></div>
          <div class="box e"><${Origins}/></div>
          <div class="box f"><${ShareFile}/></div>
          <div class="box g"><${Files}/></div>
       </div>
      </section>
    `
    }
}

render(html`<${App} />`, document.getElementById('root'));
