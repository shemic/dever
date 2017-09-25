<?php namespace Dever\Data\Elastic;

use Dever\Output\Debug;
use Dever\Http\Curl;

class Connect
{
    /**
     * handle
     *
     * @var object
     */
    private $handle;

    /**
     * curl
     *
     * @var object
     */
    private $curl;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * getInstance
     *
     * @return Dever\Data\Mongo\Connect;
     */
    public static function getInstance($config)
    {
        $key = $config['host'] . $config['database'];
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self();
            self::$instance[$key]->init($config);
        }

        return self::$instance[$key];
    }

    /**
     * init
     *
     * @return mixd
     */
    private function init($config)
    {
        if (strpos($config['host'], ':') !== false) {
            $this->host = $config['host'];
        } else {
            $this->host = $config['host'] . ':' . $config['port'];
        }

        $this->host = 'http://' . $this->host . '/';
        $this->handle = $this->host . $config['database'] . '/';
        $this->curl = new Curl();
        if (isset($config['username'])) {
            $setting['UserPWD'] = $config['username'];
            if (isset($config['password'])) {
                $setting['UserPWD'] .= ':' . $config['password'];
            }
            $this->curl->setting($setting);
        }

        Debug::log('elastic ' . $config['host'] . ' connected', $config['type']);
    }

    /**
     * __destruct
     *
     * @return mixd
     */
    public function __destruct()
    {
        $this->close();
    }

    /**
     * handle
     *
     * @return object
     */
    public function handle($url = '_status', $type = 'get', $param = array(), $state = true)
    {
        $url = $this->handle . $url;
        if (strpos($url, '/scroll')) {
            $url = $this->host . '_search/scroll';
        }
        $this->curl->load($url, $param, $type, true);
        $result = $this->curl->result(false);
        if (!$state) {
            return;
        }
        $result = json_decode($result, true);
        if (isset($result['_scroll_id'])) {
            $oper = new \Dever\Session\Oper(DEVER_PROJECT, 'cookie');
            $oper->add('es_scroll', $result['_scroll_id'], 3600);
        }
        if (isset($result['hits'])) {
            $return = array();
            $return['total'] = $result['hits']['total'];
            $return['data'] = array();
            if (isset($param['aggs']) && isset($result['aggregations'])) {
                $return['data'] = $this->aggregations($result['aggregations']);
            }
            if (!$return['data'] && $return['total'] > 0) {
                foreach ($result['hits']['hits'] as $k => $v) {
                    $return['data'][$k] = $this->hits($v);
                }
            }
            
            return $return;
        }

        if (isset($result['error'])) {
            $msg = $result['error']['root_cause'][0]['reason'];
            if ($msg != 'no such index' && strpos($msg, 'unknown setting') === false) {
                Debug::wait($msg, 'Dever NOSQL DB Error!');
            } else {
                return array();
            }
        }

        if (isset($result['_id'])) {
            return $result['_id'];
        }
        return $result;
    }

    /**
     * hits
     *
     * @return mixd
     */
    public function hits($v)
    {
        $return = $v['_source'];
        if (isset($return['@timestamp'])) {
            $return['cdate'] = $return['@timestamp'];
            unset($return['@timestamp']);
        }
        if (isset($v['_score'])) {
            $return['score'] = $v['_score'];
        }
        if (!isset($v['_source']['id']) && isset($v['_id'])) {
            $return['id'] = $v['_id'];
        }

        if (isset($v['_source']['add'])) {
            $return = array_merge($return, $v['_source']['add']);
            unset($return['add']);
        }

        if (isset($v['highlight'])) {
            foreach ($v['highlight'] as $i => $j) {
                $return['s_' . $i] = $j[0];
            }
        }
        return $return;
    }

    /**
     * aggregations
     *
     * @return mixd
     */
    public function aggregations($aggs)
    {
        $return = array();
        foreach ($aggs as $k => $v) {
            if (isset($v['buckets'])) {
                foreach ($v['buckets'] as $i => $j) {
                    $return[$i] = $j;
                    $return[$i][$k] = $j['key'];
                    unset($return[$i]['key']);
                }
            }
        }
        return $return;
    }

    /**
     * close
     *
     * @return mixd
     */
    public function close()
    {
        $this->handle = null;
    }
}
