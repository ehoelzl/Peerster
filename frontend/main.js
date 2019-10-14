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
    });
    this.setState(({newNode}) => ({newNode: ""}))
    this.refreshNodes()
    event.preventDefault()
  }

  render () {
    return html`
      <section>
        <div style="overflow: hidden">
             <div style="float: left;margin-right: 20px;font-weight: bold">Known Peers </div>
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
            <div style="font-weight: bold; margin-bottom: 20px">Add a new peer</div>
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
      var numMessages = Object.keys(this.state.messages[node]).length
      for (let _id = 1; _id <= numMessages; _id++){
        var message = this.state.messages[node][_id]
        items.push(html`<div class="message">
                        <div style="width: 200px; float: left; font-family: monospace">${node}</div>
                        <div>${message}</div>
                        </div>`)
      }
    }
    return items
  }

  render () {
    return html`
        <section>
            <div style="font-weight: bold; margin-bottom: 20px">Messages</div>
            <div style="margin-bottom: 15px; border-bottom: 2px dotted">
                <div style="width: 200px;float: left; font-family: monospace">From</div>
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
        <div style="font-weight: bold">Chat Box</div>
        <hr style="border-top: 2px"/>
        <form style="width: 70px" onSubmit=${this.handleSubmit.bind(this)}>
            <textarea value=${this.state.message} onChange=${this.handleChange.bind(this)} style="background: white; resize: none; height:150px; width: 270%"/>
            <input type="submit" value="Send" class="button"/>
        </form>
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

       </div>
      </section>
    `
  }
}

render(html`<${App} />`, document.getElementById('root'));
