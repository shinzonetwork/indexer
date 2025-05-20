use std::error::Error;
use ethabi::decode as ethabi_decode;
use ethabi::Token;
use ethereum_types::H256;
use serde::{Serialize, Deserialize};
use lens_sdk::StreamOption;
use lens_sdk::option::StreamOption::{Some, None, EndOfStream};

#[link(wasm_import_module = "lens")]
extern "C" {
    fn next() -> *mut u8;
}

#[derive(Serialize)]
pub struct Output {
    pub index_topic_0: String,
    pub index_topic_1: String,
    pub index_topic_2: String,
    pub index_topic_3: String,
    pub index_topic_4: String,
    pub value:         String,
}

#[derive(Deserialize)]
pub struct Input {
    pub topics: String,
    pub number: u64,
    pub abi: Vec<u8>,
}

#[no_mangle]
pub extern fn alloc(size: usize) -> *mut u8 {
    lens_sdk::alloc(size)
}

#[no_mangle]
pub extern fn transform() -> *mut u8 {
    match try_transform() {
        Ok(o) => match o {
            Some(result_json) => lens_sdk::to_mem(lens_sdk::JSON_TYPE_ID, &result_json),
            None => lens_sdk::nil_ptr(),
            EndOfStream => lens_sdk::to_mem(lens_sdk::EOS_TYPE_ID, &[]),
        },
        Err(e) => lens_sdk::to_mem(lens_sdk::ERROR_TYPE_ID, &e.to_string().as_bytes()),
    }
}

fn try_transform() -> Result<StreamOption<Vec<u8>>, Box<dyn Error>> {
    let ptr = unsafe { next() };
    let input = match lens_sdk::try_from_mem::<Input>(ptr)? {
        Some(v) => v,
        None => return Ok(None),
        EndOfStream => return Ok(EndOfStream),
    };

    let topics: Vec<&str> = input.topics.split(';').collect();
    let mut result = Output {
        index_topic_0: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_1: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_2: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_3: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_4: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        value: "".to_string(),
    };

    for i in 0..input.number {
        let idx = i as usize;
        if idx < topics.len() {
            match i {
                0 => result.index_topic_0 = decode_topic(topics[idx], &input.abi)?,
                1 => result.index_topic_1 = decode_topic(topics[idx], &input.abi)?,
                2 => result.index_topic_2 = decode_topic(topics[idx], &input.abi)?,
                3 => result.index_topic_3 = decode_topic(topics[idx], &input.abi)?,
                4 => result.index_topic_4 = decode_topic(topics[idx], &input.abi)?,
                _ => break,
            }
        } else {
            break;
        }
    }

    let result_json = serde_json::to_vec(&result)?;
    lens_sdk::free_transport_buffer(ptr)?;
    Ok(Some(result_json))
}

fn decode_topic(topic: &str, abi: &[u8]) -> Result<String, Box<dyn Error>> {
    // Parse the ABI
    let parsed_abi = ethabi::Contract::load(abi)?;
    
    // Convert hex topic to H256
    let topic_bytes = hex::decode(topic.trim_start_matches("0x"))?;
    let mut bytes = [0u8; 32];
    bytes.copy_from_slice(&topic_bytes);
    let topic_hash = H256::from(bytes);
    
    // Find the event in the ABI and decode
    for event in parsed_abi.events() {
        if let Ok(decoded) = event.parse_log(ethabi::RawLog {
            topics: vec![topic_hash],
            data: vec![],
        }) {
            return Ok(decoded.params.iter()
                .map(|param| param.value.to_string())
                .collect::<Vec<_>>().join(", "));
        }
    }
    
    // If we couldn't decode it, return the original topic
    Ok(topic.to_string())
}

