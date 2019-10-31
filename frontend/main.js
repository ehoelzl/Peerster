const { h, Component, render } = preact;
const html = htm.bind(h);
const uiAddress = "http://127.0.0.1:8080"

class Node extends Component {
  state = {nodeString: ""};

  refreshNode(){
    $.getJSON(uiAddress + "/id", function(data) {
      this.setState(({gossipAddress, name}) => (data))
      this.setState(({disconnected}) => ({disconnected: false}))
    }.bind(this)).fail(function () {
      this.setState({disconnected: true, name: "", gossipAddress: ""})
    }.bind(this))
  }

  componentDidMount(){
    this.refreshNode()
    setInterval(() => this.refreshNode(), 15*1000)
  }

  NodeString(props){
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


  render () {
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
  state = {nodes : [], newNode: ""};

  refreshNodes() {
    $.getJSON(uiAddress + '/node', function(data) {
      this.setState(({nodes}) => ({nodes: data}))
    }.bind(this)).fail(function()  {
      this.setState(({nodes}) => ({nodes : []}))
    }.bind(this))
  }

  handleChange(event) {
    this.setState(({ newNode }) => ({newNode : event.target.value}))
  }

  componentDidMount(){
    this.refreshNodes()
    setInterval(() => this.refreshNodes(), 15*1000)
  }

  handleSubmit(event){

    $.ajax({
      type: 'POST',
      url: uiAddress + '/node',
      async: false,
      data: JSON.stringify({"Text": this.state.newNode}),
      contentType: "application/json",
      dataType: 'json'
    }).fail((d, ts, xhr) => {
      if (d.status == 400){
        alert("Could not resolve address")
      }
    });
    this.setState(({newNode}) => ({newNode: ""}))
    this.refreshNodes()
    event.preventDefault()
  }

  render () {
    return html`
      <section>
        <div style="overflow: hidden">
             <div style="float: left;margin-right: 20px;font-weight: bold">Nodes</div>
             <div style="float: left" class="is-special"> ${this.state.nodes.length}</div>
        </div>
        <hr style="border-top: 2px"/>
        <div class="peers" style="margin-bottom: 20px">
            ${this.state.nodes.map(node => {
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
  state = {messages : {}}

  refreshMessages() {
    $.getJSON(uiAddress + '/message', function(data) {
      this.setState(({messages}) => ({messages: data}))
    }.bind(this))
  }

  componentDidMount(){
    this.refreshMessages();
    setInterval(() => this.refreshMessages(), 1*1000)
  }

  printNodeMessages() {
    var items = [];
    for (var node in this.state.messages){
        var messageIds = Object.keys(this.state.messages[node]).sort()
        var numMessages = messageIds.length
        for (let _id = 0; _id < numMessages; _id++){
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

  render () {
    return html`
        <section>
            <div style="font-weight: bold; margin-bottom: 20px">Rumor Messages</div>
            <div style="margin-bottom: 15px; border-bottom: 2px dotted">
                <div style="width: 220px;float: left; font-family: monospace">From</div>
                <div>Message</div>
            </div>
            <div class="messages">
                ${this.printNodeMessages()}        
            </div>
        </section>
    `
  }
}

class ChatBox extends Component {
  state  = {message: ""};

  handleChange(event){
    this.setState(({ message }) => ({message : event.target.value}))
  }

  handleSubmit(event){
    event.preventDefault()
    if (this.state.message.length == 0 ){
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
    this.setState(({message}) => ({message : ""}));
  }

  render () {
    return html `
        <div style="font-weight: bold">Rumor Chat Box</div>
        <hr style="border-top: 2px"/>
        <form style="width: 100px" onSubmit=${this.handleSubmit.bind(this)}>
            <textarea value=${this.state.message} onChange=${this.handleChange.bind(this)} style="background: white; resize: none; height:150px; width: 270%"/>
            <input type="submit" value="Send" class="button"/>
        </form>
    `
  }
}

class Popup extends Component {
  state = {message : ""}

  handleChange() {
    this.setState(({ message }) => ({message : event.target.value}))
  }
  render() {
    return html`
        <div class='popup'>
          <div class='popup\_inner'>
            <div style="text-align: center; margin-top: 30px; margin-bottom: 15px">Send private message to ${this.props.destination}</div>
            <textarea value=${this.state.message} onChange=${this.handleChange.bind(this)} style="background: white; width: 50%; height: 100px; resize: none"/>
            <p>
            <button onClick=${() => this.props.send(this.props.destination, this.state.message)} style="width: 200px;">Send</button>
            <button onClick=${this.props.cancel} style="width: 200px">Cancel</button>
            </p>
          </div>
        </div>
    `
  }
}

class PrivateMessages extends Component {
  state = {peers: [], popUp: false, destination : null};

  refreshOrigins() {
    $.getJSON(uiAddress + '/origins', function(data) {
      if (data != null){
        this.setState(({peers}) => ({peers: data.sort()}))
      }
    }.bind(this)).fail(function()  {
      this.setState(({peers}) => ({peers : []}))
    }.bind(this))
  }

  componentDidMount(){
    this.refreshOrigins()
    setInterval(() => this.refreshOrigins(), 15*1000)
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
        success: alert("Message sent successfully")
      }).fail((d, ts, xhr) => {
        if (d.status == 400){
          alert("Could not send private message")
        }
      })
    }
    this.togglePopUp()
  }

  renderPeerButton(peer) {
    return html`
          <div class="message">
            <div style="width: 50%;font-family: monospace; float: left; height: 100%">${peer}</div>
            <div style="width: 50%; float: right"><button onClick=${() => {
              this.setState(({destination}) => ({destination: peer}));
              this.togglePopUp()
      
              }}>Send Private Message</button></div>
            </div>
    `
  }

  togglePopUp() {
    this.setState(({popUp}) => ({popUp: !this.state.popUp}))
  }

  renderPopUp() {
    if (this.state.popUp) {
      return html`<${Popup} destination=${this.state.destination} send=${this.sendPrivateMessage.bind(this)} cancel=${this.togglePopUp.bind(this)}/>`
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
        <div class="peers" style="margin-bottom: 20px">
            ${this.state.peers.map(p => this.renderPeerButton(p))}
        </div>
      </section>
    `

  }
}
class App extends Component {
  render () {
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
          <div class="box e"><${PrivateMessages}/></div>
       </div>
      </section>
    `
  }
}

render(html`<${App} />`, document.getElementById('root'));
